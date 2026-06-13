package daemonapi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	reqh2 "github.com/opensvc/om3/v3/core/client/requester/h2"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// GetNodeMetrics proxies to the node's /metrics endpoint and filters
// the metrics based on the requester's RBAC grants.
//
// Access rules:
// - Guest role on the namespace or all namespaces or better is sufficient to access object metrics.
// - Root role is necessary to access daemon metrics.
func (a *DaemonAPI) GetNodeMetrics(ctx echo.Context, nodename api.InPathNodeName) error {
	// Parse and validate nodename
	nodename = a.parseNodename(nodename)

	// Check if we can access any metrics on this node
	// At minimum, we need guest role on any namespace to access object metrics
	// OR root role to access daemon metrics
	userGrants := grantsFromContext(ctx)

	hasObjectAccess := userGrants.HasRoleOn("", rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin) ||
		len(userGrants.Namespaces(rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin)) > 0

	hasDaemonAccess := userGrants.HasRole(rbac.RoleRoot)

	if !hasObjectAccess && !hasDaemonAccess {
		return JSONForbiddenMissingRole(ctx, rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin, rbac.RoleRoot)
	}

	// For local node, shortcut to the raw /metrics endpoint with filtering
	if a.localhost == nodename {
		return a.serveLocalMetrics(ctx, userGrants)
	}

	// For remote nodes, proxy to the remote node's GetNodeMetrics endpoint
	// which will do the filtering on the remote side
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeMetrics(ctx.Request().Context(), nodename)
	})
}

// serveLocalMetrics serves metrics for the local node with RBAC filtering
func (a *DaemonAPI) serveLocalMetrics(ctx echo.Context, userGrants rbac.Grants) error {
	// For local node, use newProxyClient to get a client with UDS configured
	c := reqh2.NewUDSClient(reqh2.Config{
		Timeout: 1 * time.Second,
	})

	// Create request to /metrics endpoint
	req, err := http.NewRequestWithContext(ctx.Request().Context(), http.MethodGet, "http://localhost/metrics", nil)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Create request", "%s", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Get metrics", "%s", err)
	}
	defer resp.Body.Close()

	// If user has root role, return all metrics without filtering
	if userGrants.HasRole(rbac.RoleRoot) {
		// Copy headers and stream the response
		for key, values := range resp.Header {
			for _, v := range values {
				ctx.Response().Header().Add(key, v)
			}
		}
		return ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
	}

	// Filter the metrics based on namespace access
	filteredResp, err := filterMetricsResponse(resp, userGrants)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Filter metrics", "%s", err)
	}
	defer filteredResp.Body.Close()

	// Copy headers and stream the response
	for key, values := range filteredResp.Header {
		for _, v := range values {
			ctx.Response().Header().Add(key, v)
		}
	}

	return ctx.Stream(filteredResp.StatusCode, filteredResp.Header.Get("Content-Type"), filteredResp.Body)
}

// filterMetricsResponse reads the metrics response, filters it based on user grants,
// and returns a new response with only the allowed metrics
func filterMetricsResponse(resp *http.Response, userGrants rbac.Grants) (*http.Response, error) {
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body.Close()

	// Get the filtered metrics
	filteredMetrics := filterMetricsText(string(body), userGrants)

	// Create a new response with the filtered content
	filteredResp := &http.Response{
		StatusCode:    resp.StatusCode,
		Status:        resp.Status,
		Header:        resp.Header.Clone(),
		Body:          io.NopCloser(bytes.NewReader([]byte(filteredMetrics))),
		ContentLength: int64(len(filteredMetrics)),
	}

	return filteredResp, nil
}

// filterMetricsText filters Prometheus metrics text based on user grants
func filterMetricsText(metricsText string, userGrants rbac.Grants) string {
	// If user has root role, return all metrics
	if userGrants.HasRole(rbac.RoleRoot) {
		return metricsText
	}

	// Parse allowed namespaces for object metrics
	// User can see object metrics for namespaces they have guest role or better
	allowedNamespaces := userGrants.Namespaces(rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin)

	// Also check for global namespace grants (empty string means all namespaces)
	if userGrants.HasRoleOn("", rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin) {
		// User has access to all namespaces
		return metricsText // If user has global guest/admin/operator, show all object metrics
	}

	// If user has no namespace access, only daemon metrics would be shown (but they don't have root)
	// So in this case, return empty
	if len(allowedNamespaces) == 0 {
		return ""
	}

	// Patterns
	objectMetricPattern := regexp.MustCompile(`^opensvc_pg_`)
	namespaceLabelPattern := regexp.MustCompile(`namespace="([^"]+)"`)

	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(metricsText))

	// State tracking for multi-line metric samples
	var keepCurrentMetric bool

	for scanner.Scan() {
		line := scanner.Text()

		// Handle empty lines
		if strings.TrimSpace(line) == "" {
			if keepCurrentMetric {
				result.WriteString(line + "\n")
			}
			continue
		}

		// Handle comment lines (TYPE, HELP, etc.)
		if strings.HasPrefix(line, "#") {
			// Extract metric name from TYPE and HELP comments
			// Format: # TYPE metric_name type
			// Format: # HELP metric_name help text
			if strings.HasPrefix(line, "# TYPE ") || strings.HasPrefix(line, "# HELP ") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					metricName := parts[2]
					// Check if this is an object metric
					if objectMetricPattern.MatchString(metricName) {
						// For TYPE and HELP comments of object metrics, keep them
						// We'll determine later if we keep the actual samples
						result.WriteString(line + "\n")
						keepCurrentMetric = true
					} else {
						// Daemon metric TYPE/HELP - only keep if user has root (but we already checked)
						keepCurrentMetric = false
					}
				}
			} else {
				// Other comments - keep them for context
				result.WriteString(line + "\n")
			}
			continue
		}

		// Handle metric sample lines
		// Format: metric_name{label1="val1",label2="val2"} value
		// or: metric_name value (no labels)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Continuation line (shouldn't happen in standard Prometheus output)
			// or indented sample line
			if keepCurrentMetric {
				result.WriteString(line + "\n")
			}
			continue
		}

		// This is a metric sample line: metric_name{labels} value
		// First, check if it's an object metric
		isObjectMetric := objectMetricPattern.MatchString(line)

		if isObjectMetric {
			// Extract namespace from labels
			namespace := extractNamespaceFromLine(line, namespaceLabelPattern)

			if namespace != "" {
				// Check if this namespace is allowed
				keepCurrentMetric = isNamespaceAllowed(namespace, allowedNamespaces)
			} else {
				// No namespace label found in this line
				// This could be a metric without labels or a metric declaration
				// For object metrics without namespace labels, we can't filter by namespace
				// So we'll keep them only if user has any namespace access
				keepCurrentMetric = true
			}

			if keepCurrentMetric {
				result.WriteString(line + "\n")
			}
		} else {
			// This is a daemon metric - skip (user doesn't have root)
			keepCurrentMetric = false
		}
	}

	return result.String()
}

// extractNamespaceFromLine extracts the namespace value from a metric line
func extractNamespaceFromLine(line string, pattern *regexp.Regexp) string {
	matches := pattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isNamespaceAllowed checks if a namespace is in the allowed list
func isNamespaceAllowed(ns string, allowedNamespaces []string) bool {
	// If "*" is in the list, all namespaces are allowed
	for _, allowed := range allowedNamespaces {
		if allowed == "*" {
			return true
		}
	}

	// Check if the namespace is in the allowed list
	for _, allowed := range allowedNamespaces {
		if ns == allowed {
			return true
		}
	}

	return false
}
