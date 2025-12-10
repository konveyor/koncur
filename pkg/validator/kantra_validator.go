package validator

// kantraValidator implements the standard kantra validation behavior
// It embeds baseValidator and uses all its default implementations
type kantraValidator struct {
	baseValidator
}
