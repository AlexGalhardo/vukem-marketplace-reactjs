package requests

// Validation messages that are not built into z are i18n keys; the
// account handlers translate them into the request locale.
const (
	MsgPasswordMismatch = "account.errors.password_mismatch"
	MsgCodeRequired     = "account.errors.code_required"
)
