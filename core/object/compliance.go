package object

import "opensvc.com/opensvc/util/compliance"

type (
	Compliancer interface {
		ComplianceAttachModuleset(options OptsObjectComplianceAttachModuleset) error
		ComplianceAttachRuleset(options OptsObjectComplianceAttachRuleset) error
		ComplianceAuto(options OptsObjectComplianceAuto) (*compliance.Run, error)
		ComplianceCheck(options OptsObjectComplianceCheck) (*compliance.Run, error)
		ComplianceDetachModuleset(options OptsObjectComplianceDetachModuleset) error
		ComplianceDetachRuleset(options OptsObjectComplianceDetachRuleset) error
		ComplianceEnv(options OptsObjectComplianceEnv) (compliance.Envs, error)
		ComplianceFixable(options OptsObjectComplianceFixable) (*compliance.Run, error)
		ComplianceFix(options OptsObjectComplianceFix) (*compliance.Run, error)
		ComplianceListModuleset(options OptsObjectComplianceListModuleset) ([]string, error)
		ComplianceListModules() ([]string, error)
		ComplianceListRuleset(options OptsObjectComplianceListRuleset) ([]string, error)
		ComplianceShowModuleset(options OptsObjectComplianceShowModuleset) (*compliance.ModulesetTree, error)
		ComplianceShowRuleset() (compliance.Rulesets, error)
	}
)
