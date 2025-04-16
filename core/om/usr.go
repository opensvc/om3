package om

import "github.com/opensvc/om3/core/commoncmd"

func init() {
	kind := "usr"

	cmdObject := newCmdUsr()
	cmdObjectCertificate := newCmdObjectCertificate(kind)
	cmdObjectConfig := newCmdObjectConfig(kind)
	cmdObjectEdit := newCmdObjectEdit(kind)
	cmdObjectKey := newCmdObjectKey(kind)
	cmdObjectInstance := newCmdObjectInstance(kind)
	cmdObjectSet := newCmdObjectSet(kind)
	cmdObjectPrint := newCmdObjectPrint(kind)
	cmdObjectPrintConfig := newCmdObjectPrintConfig(kind)
	cmdObjectValidate := newCmdObjectValidate(kind)

	root.AddCommand(
		cmdObject,
	)
	cmdObject.AddCommand(
		cmdObjectCertificate,
		cmdObjectConfig,
		cmdObjectEdit,
		cmdObjectKey,
		cmdObjectInstance,
		cmdObjectPrint,
		cmdObjectSet,
		cmdObjectValidate,
		newCmdDataStoreAdd(kind),
		newCmdDataStoreChange(kind),
		newCmdDataStoreDecode(kind),
		newCmdDataStoreKeys(kind),
		newCmdDataStoreRemove(kind),
		newCmdDataStoreRename(kind),
		newCmdObjectCreate(kind),
		newCmdObjectDelete(kind),
		newCmdObjectEval(kind),
		newCmdObjectGet(kind),
		newCmdObjectLogs(kind),
		newCmdObjectList(kind),
		commoncmd.NewCmdObjectMonitor("", kind),
		newCmdObjectPurge(kind),
		newCmdObjectUnset(kind),
		newCmdObjectGenCert(kind),
		newCmdObjectPKCS(kind),
	)
	cmdObjectCertificate.AddCommand(
		newCmdObjectCertificateCreate(kind),
		newCmdObjectCertificatePKCS(kind),
	)
	cmdObjectConfig.AddCommand(
		newCmdObjectConfigDoc(kind),
		newCmdObjectConfigEdit(kind),
		newCmdObjectConfigEval(kind),
		newCmdObjectConfigGet(kind),
		newCmdObjectConfigMtime(kind),
		newCmdObjectConfigShow(kind),
		newCmdObjectConfigUpdate(kind),
		newCmdObjectConfigValidate(kind),
	)
	cmdObjectEdit.AddCommand(
		newCmdObjectEditConfig(kind),
	)
	cmdObjectKey.AddCommand(
		newCmdObjectKeyAdd(kind),
		newCmdObjectKeyChange(kind),
		newCmdObjectKeyDecode(kind),
		newCmdObjectKeyEdit(kind),
		newCmdObjectKeyInstall(kind),
		newCmdObjectKeyList(kind),
		newCmdObjectKeyRemove(kind),
		newCmdObjectKeyRename(kind),
	)
	cmdObjectInstance.AddCommand(
		newCmdObjectInstanceList(kind),
	)
	cmdObjectPrint.AddCommand(
		cmdObjectPrintConfig,
	)
	cmdObjectPrintConfig.AddCommand(
		newCmdObjectConfigMtime(kind),
	)
	cmdObjectValidate.AddCommand(
		newCmdObjectValidateConfig(kind),
	)
}
