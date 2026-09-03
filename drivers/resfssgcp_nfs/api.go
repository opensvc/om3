package resfssgcp_nfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/util/ageingcache"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sgcp"
)

type (
	// nfsClientMgr handles NFS client management for the resource
	nfsClientMgr struct {
		uuid       string
		host       string
		permission string
		protocol   string
		nfsIgnored []string

		api         *sgcp.FilesAPI
		cacheConfig *sgcp.CacheConfig
		log         *plog.Logger
	}
)

// Start adds the NFS client to the filesystem
func (mgr *nfsClientMgr) Start(ctx context.Context, exclusive bool) error {
	if exclusive {
		return mgr.startExclusive(ctx)
	}
	return mgr.start(ctx)
}

func (mgr *nfsClientMgr) start(ctx context.Context) error {
	// Verify if the client already exists
	fileInfo, err := mgr.getFileInfo(ctx)
	if err != nil {
		return err
	}

	if fileInfo != nil {
		for _, client := range fileInfo.NFSClients {
			if client.Host == mgr.host {
				if client.Permission == mgr.permission {
					mgr.log.Infof("%s already exists", client)
					return nil
				}
				// Wrong permission, delete and recreate
				if err := mgr.deleteNFSClient(ctx, client); err != nil {
					// Don't stop here, next addNFSClient will retry delete wrong permission
					mgr.log.Debugf("continue (will be retried) on failed deletion of NFS client with wrong permission: %s", err)
				}
				break
			}
		}
	}

	// Add the client
	_, err = mgr.addNFSClient(ctx, mgr.host)
	return err
}

// startExclusive starts with exclusive access (removes other clients first)
func (mgr *nfsClientMgr) startExclusive(ctx context.Context) error {
	// Delete all clients except our host
	fileInfo, err := mgr.getFileInfo(ctx)
	if err != nil {
		return err
	}

	if fileInfo != nil {
		for _, client := range fileInfo.NFSClients {
			if client.Host != mgr.host && !mgr.isClientIgnored(client.Host) {
				if err := mgr.deleteNFSClient(ctx, client); err != nil {
					return err
				}
			}
		}
	}

	// Add our client
	_, err = mgr.addNFSClient(ctx, mgr.host)
	return err
}

func (mgr *nfsClientMgr) cacheSig(name string) string {
	return fmt.Sprintf("%s:%s", name, mgr.uuid)
}

func (mgr *nfsClientMgr) cacheClear(name string) error {
	cacheSig := mgr.cacheSig(name)
	return ageingcache.Clear(cacheSig)
}

func (mgr *nfsClientMgr) getFileInfo(ctx context.Context) (*FilesystemInfo, error) {
	var fileInfo FilesystemInfo

	cacheSig := mgr.cacheSig("getFileInfo")
	ttl := time.Duration(mgr.cacheConfig.TTLSeconds) * time.Second
	o := ageingcache.NewOutputter(mgr.getFileInfoFactory(ctx))
	data, err := ageingcache.Output(o, cacheSig, ttl)
	if err != nil {
		mgr.log.Debugf("getFileInfo failed on missing cache sig %s.out, ttl: %s: %s", cacheSig, ttl, err)
		return nil, fmt.Errorf("getFileInfo failed: %w", err)
	}

	// The factory records an absent filesystem as a null document, the only
	// way for it to travel through the cache. Hand it back as no filesystem
	// at all, which is what every caller tests for.
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	if err := json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}

	return &fileInfo, err
}

func (mgr *nfsClientMgr) getFileInfoFactory(ctx context.Context) func() ([]byte, error) {
	return func() ([]byte, error) {
		method, url, statusCode, data, err := mgr.api.GetFilesystem(ctx, mgr.uuid)

		// The status code is read before the error: the api reports every
		// status over 400 as one, and a filesystem that is not there is an
		// answer. Cache it as a null document, so the absence ages like a
		// presence would.
		if statusCode == http.StatusNotFound {
			mgr.log.Debugf("%s %s: no such filesystem", method, url)
			return []byte("null"), nil
		}
		if err != nil {
			return nil, err
		}
		if err := mgr.api.CheckStatusCode(method, url, statusCode, http.StatusOK); err != nil {
			return nil, err
		}
		return data, nil
	}
}

// Stop removes the NFS client for this host
func (mgr *nfsClientMgr) Stop(ctx context.Context) error {
	fileInfo, err := mgr.getFileInfo(ctx)
	if err != nil {
		return err
	}

	if fileInfo == nil {
		return nil
	}

	for _, client := range fileInfo.NFSClients {
		if client.Host == mgr.host {
			return mgr.deleteNFSClient(ctx, client)
		}
	}

	return nil
}

// addNFSClient adds a new NFS client to the filesystem
func (mgr *nfsClientMgr) addNFSClient(ctx context.Context, host string) (*NfsClient, error) {
	newClient := &NfsClient{
		Host:       host,
		Permission: mgr.permission,
		Protocol:   mgr.protocol,
	}

	mgr.log.Debugf("Verify existence of %s", newClient)

	fileInfo, err := mgr.getFileInfo(ctx)
	if err != nil {
		return nil, err
	}

	if fileInfo != nil {
		for _, client := range fileInfo.NFSClients {
			if client.Host == host {
				if client.Permission == mgr.permission {
					mgr.log.Infof("%s already exists", client)
					return &client, nil
				}
				mgr.log.Infof("%s already exists, with wrong permission", client)
				if err := mgr.deleteNFSClient(ctx, client); err != nil {
					mgr.log.Warnf("delete existing %s with wrong permissions: %s", client, err)
				}
				break
			}
		}
	}
	payload := &sgcp.NfsClient{
		Host:       host,
		Permission: mgr.permission,
		Protocol:   mgr.protocol,
	}
	mgr.log.Infof("grant permission %s for host %s on filesystem %s ...", mgr.permission, host, mgr.uuid)
	method, url, statusCode, data, err := mgr.api.PostNFSClients(ctx, mgr.uuid, payload)

	if err != nil {
		return nil, err
	}
	if err := mgr.api.CheckStatusCode(method, url, statusCode, http.StatusCreated); err != nil {
		return nil, err
	}
	if statusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %s %s: %d", method, url, statusCode)
	}
	var createdClient NfsClient
	if err := json.Unmarshal(data, &createdClient); err != nil {
		return nil, err
	}

	mgr.log.Infof("created %s on filesystem %s", createdClient, mgr.uuid)
	return &createdClient, nil
}

// deleteNFSClient deletes an NFS client from the filesystem
func (mgr *nfsClientMgr) deleteNFSClient(ctx context.Context, client NfsClient) error {
	cgMsg := ""
	if client.ConsistencyGroupID != "" {
		cgMsg = fmt.Sprintf(" in consistency group %s", client.ConsistencyGroupID)
	}

	mgr.log.Infof("drop permission %s for host %s on filesystem %s%s ...", client.Permission, client.Host, mgr.uuid, cgMsg)
	method, url, statusCode, _, err := mgr.api.DeleteNFSClients(ctx, mgr.uuid, client.UUID)

	// Read before the error, as above: the provider refusing the drop while
	// the consistency group is busy is a state to report, not a transport
	// failure.
	if statusCode == http.StatusPreconditionFailed {
		return fmt.Errorf("consistency group is not in ready (status_code %d)", statusCode)
	}
	if err != nil {
		return err
	}
	if err := mgr.api.CheckStatusCode(method, url, statusCode, http.StatusNoContent); err != nil {
		return err
	}

	mgr.log.Infof("deleted %s on filesystem %s", client, mgr.uuid)
	return nil
}

// isClientIgnored checks if a client host should be ignored
func (mgr *nfsClientMgr) isClientIgnored(host string) bool {
	for _, ignored := range mgr.nfsIgnored {
		if host == ignored {
			return true
		}
	}
	return false
}
