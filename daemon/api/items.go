package api

// The lists the api answers with wrap their items in a Items field, and
// the output renderer asks for that field through this interface rather
// than reaching into the struct. It is the one thing the renderer
// cannot read from the json tags: which of the fields holds the rows.

func (t CapabilityList) GetItems() any {
	return t.Items
}

func (t DiskList) GetItems() any {
	return t.Items
}

func (t DriverList) GetItems() any {
	return t.Items
}

func (t ScheduleList) GetItems() any {
	return t.Items
}

func (t NodeList) GetItems() any {
	return t.Items
}

func (t GroupList) GetItems() any {
	return t.Items
}

func (t HardwareList) GetItems() any {
	return t.Items
}

func (t IPAddressList) GetItems() any {
	return t.Items
}

func (t InstanceList) GetItems() any {
	return t.Items
}

func (t KeywordList) GetItems() any {
	return t.Items
}

func (t PackageList) GetItems() any {
	return t.Items
}

func (t PropertyList) GetItems() any {
	return t.Items
}

func (t ObjectList) GetItems() any {
	return t.Items
}

func (t ResourceList) GetItems() any {
	return t.Items
}

func (t UserList) GetItems() any {
	return t.Items
}

func (t SANPathList) GetItems() any {
	return t.Items
}

func (t SANPathInitiatorList) GetItems() any {
	return t.Items
}

func (t NetworkIPList) GetItems() any {
	return t.Items
}

func (t NetworkList) GetItems() any {
	return t.Items
}

func (t PoolList) GetItems() any {
	return t.Items
}

func (t PoolVolumeList) GetItems() any {
	return t.Items
}

func (t DataKeyList) GetItems() any {
	return t.Items
}

func (t RelayStatusList) GetItems() any {
	return t.Items
}

func (t ResourceInfoList) GetItems() any {
	return t.Items
}
