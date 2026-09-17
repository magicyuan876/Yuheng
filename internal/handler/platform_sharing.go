package handler

// SetSharingRequest is the body of every PUT .../{id}/sharing endpoint.
//
// A pointer so an omitted field is rejected by binding rather than silently
// read as false: "stop sharing" and "field absent" must not look the same on
// an endpoint whose whole job is flipping one boolean.
type SetSharingRequest struct {
	Shared *bool `json:"shared" binding:"required"`
}
