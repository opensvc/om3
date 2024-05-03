package arrayfreenas

// Pool defines model for Pool.
type Pool struct {
	EncryptkeyPath *string `json:"encryptkey_path,omitempty"`
	Guid           *string `json:"guid,omitempty"`
	Healthy        *bool   `json:"healthy,omitempty"`
	Id             int     `json:"id"`
	IsDecrypted    *bool   `json:"is_decrypted,omitempty"`
	Name           string  `json:"name"`
	Path           string  `json:"path"`
	Status         *string `json:"status,omitempty"`
}

// PoolsResponse defines model for PoolsResponse.
type PoolsResponse = []Pool

// GetPoolsParams defines parameters for GetPools.
type GetPoolsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}
