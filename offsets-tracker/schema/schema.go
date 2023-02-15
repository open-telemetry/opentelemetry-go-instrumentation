package schema

type TrackedOffsets struct {
	Data []TrackedLibrary `json:"data"`
}

type TrackedLibrary struct {
	Name        string              `json:"name"`
	DataMembers []TrackedDataMember `json:"data_members"`
}

type TrackedDataMember struct {
	Struct  string            `json:"struct"`
	Field   string            `json:"field_name"`
	Offsets []VersionedOffset `json:"offsets"`
}

type VersionedOffset struct {
	Offset  uint64 `json:"offset"`
	Version string `json:"version"`
}
