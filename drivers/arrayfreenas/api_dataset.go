package arrayfreenas

// CreateDatasetParams defines model for CreateDatasetParams.
type CreateDatasetParams struct {
	Aclmode           *string                               `json:"aclmode,omitempty"`
	Atime             *string                               `json:"atime,omitempty"`
	Casesensitivity   *string                               `json:"casesensitivity,omitempty"`
	Comments          *string                               `json:"comments,omitempty"`
	Compression       *string                               `json:"compression,omitempty"`
	Copies            *int                                  `json:"copies,omitempty"`
	Deduplication     *string                               `json:"deduplication,omitempty"`
	Encryption        *bool                                 `json:"encryption,omitempty"`
	EncryptionOptions *CreateDatasetParamsEncryptionOptions `json:"encryption_options,omitempty"`
	Exec              *string                               `json:"exec,omitempty"`
	ForceSize         *bool                                 `json:"force_size,omitempty"`
	InheritEncryption *bool                                 `json:"inherit_encryption,omitempty"`
	Sparse            *bool                                 `json:"sparse,omitempty"`
	Name              string                                `json:"name"`
	Quota             *int64                                `json:"quota,omitempty"`
	QuotaCritical     *int64                                `json:"quota_critical,omitempty"`
	QuotaWarning      *int64                                `json:"quota_warning,omitempty"`
	Readonly          *string                               `json:"readonly,omitempty"`
	Recordsize        *string                               `json:"recordsize,omitempty"`
	Refquota          *int64                                `json:"refquota,omitempty"`
	RefquotaCritical  *int64                                `json:"refquota_critical,omitempty"`
	RefquotaWarning   *int64                                `json:"refquota_warning,omitempty"`
	Refreservation    *int64                                `json:"refreservation,omitempty"`
	Reservation       *int64                                `json:"reservation,omitempty"`
	ShareType         *string                               `json:"share_type,omitempty"`
	Snapdir           *string                               `json:"snapdir,omitempty"`
	Sync              *string                               `json:"sync,omitempty"`
	Type              *string                               `json:"type,omitempty"`
	Volblocksize      *string                               `json:"volblocksize,omitempty"`
	Volsize           *int64                                `json:"volsize,omitempty"`
}

// CreateDatasetParamsEncryptionOptions defines model for CreateDatasetParams_encryption_options.
type CreateDatasetParamsEncryptionOptions struct {
	Algorithm   *string `json:"algorithm,omitempty"`
	GenerateKey *bool   `json:"generate_key,omitempty"`
	Key         *string `json:"key,omitempty"`
	Passphrase  *string `json:"passphrase,omitempty"`
}

// UpdateDatasetParams defines model for UpdateDatasetParams.
type UpdateDatasetParams struct {
	Aclmode        *string `json:"aclmode,omitempty"`
	Atime          *string `json:"atime,omitempty"`
	Comments       *string `json:"comments,omitempty"`
	Compression    *string `json:"compression,omitempty"`
	Copies         *int    `json:"copies,omitempty"`
	Deduplication  *string `json:"deduplication,omitempty"`
	Exec           *string `json:"exec,omitempty"`
	ForceSize      *bool   `json:"force_size,omitempty"`
	Quota          *int64  `json:"quota,omitempty"`
	Readonly       *string `json:"readonly,omitempty"`
	Recordsize     *string `json:"recordsize,omitempty"`
	Refquota       *int64  `json:"refquota,omitempty"`
	Refreservation *int64  `json:"refreservation,omitempty"`
	Snapdir        *string `json:"snapdir,omitempty"`
	Sync           *string `json:"sync,omitempty"`
	Volsize        *int64  `json:"volsize,omitempty"`
}

// Dataset defines model for Dataset.
type Dataset struct {
	Aclmode             *CompositeValue `json:"aclmode,omitempty"`
	Acltype             *CompositeValue `json:"acltype,omitempty"`
	Atime               *CompositeValue `json:"atime,omitempty"`
	Available           *CompositeValue `json:"available,omitempty"`
	Casesensitivity     *CompositeValue `json:"casesensitivity,omitempty"`
	Comments            *CompositeValue `json:"comments,omitempty"`
	Compression         *CompositeValue `json:"compression,omitempty"`
	Copies              *CompositeValue `json:"copies,omitempty"`
	Deduplication       *CompositeValue `json:"deduplication,omitempty"`
	Encrypted           *bool           `json:"encrypted,omitempty"`
	EncryptionAlgorithm *CompositeValue `json:"encryption_algorithm,omitempty"`
	EncryptionRoot      *string         `json:"encryption_root,omitempty"`
	Exec                *CompositeValue `json:"exec,omitempty"`
	Id                  string          `json:"id"`
	KeyFormat           *CompositeValue `json:"key_format,omitempty"`
	KeyLoaded           *bool           `json:"key_loaded,omitempty"`
	Locked              *bool           `json:"locked,omitempty"`
	Managedby           *CompositeValue `json:"managedby,omitempty"`
	Mountpoint          *string         `json:"mountpoint,omitempty"`
	Name                string          `json:"name"`
	Origin              *CompositeValue `json:"origin,omitempty"`
	Pbkdf2iters         *CompositeValue `json:"pbkdf2iters,omitempty"`
	Pool                string          `json:"pool"`
	Quota               *CompositeValue `json:"quota,omitempty"`
	QuotaCritical       *CompositeValue `json:"quota_critical,omitempty"`
	QuotaWarning        *CompositeValue `json:"quota_warning,omitempty"`
	Readonly            *CompositeValue `json:"readonly,omitempty"`
	Recordsize          *CompositeValue `json:"recordsize,omitempty"`
	Refquota            *CompositeValue `json:"refquota,omitempty"`
	RefquotaCritical    *CompositeValue `json:"refquota_critical,omitempty"`
	RefquotaWarning     *CompositeValue `json:"refquota_warning,omitempty"`
	Refreservation      *CompositeValue `json:"refreservation,omitempty"`
	Reservation         *CompositeValue `json:"reservation,omitempty"`
	Snapdir             *CompositeValue `json:"snapdir,omitempty"`
	Sync                *CompositeValue `json:"sync,omitempty"`
	Type                string          `json:"type"`
	Used                *CompositeValue `json:"used,omitempty"`
	Volblocksize        *CompositeValue `json:"volblocksize,omitempty"`
	Volsize             *CompositeValue `json:"volsize,omitempty"`
	Xattr               *CompositeValue `json:"xattr,omitempty"`
}

type Datasets []Dataset

// GetDatasetParams defines parameters for GetDataset.
type GetDatasetParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

// GetDatasetsParams defines parameters for GetDatasets.
type GetDatasetsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

func (t Datasets) GetByName(name string) (*Dataset, bool) {
	for _, e := range t {
		if e.Name == name {

			return &e, true
		}
	}
	return nil, false
}
