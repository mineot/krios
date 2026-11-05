package expressions

// Identity is an opaque, non-zero-sized token shared by a client's operands.
// A zero value is invalid. Tokens are compared by pointer identity.
type Identity struct{ valid bool }

func NewIdentity() *Identity {
	return &Identity{valid: true}
}

func ValidIdentity(id *Identity) bool {
	return id != nil && id.valid
}
