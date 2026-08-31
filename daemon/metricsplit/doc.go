// Package metricsplit holds no code. It exists so that the split of the
// daemon's metrics between the default prometheus registry and the per
// subsystem ones can be tested from a place that is allowed to import
// every subsystem that registers, which none of them may do of each
// other.
package metricsplit
