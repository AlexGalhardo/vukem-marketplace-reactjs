package requests

import "github.com/AlexGalhardo/vukem/z"

// UpdateProfile is the profile form. Password (the current one) is only
// required when the email changes.
type UpdateProfile struct {
	Name     string `json:"name"     form:"name"`
	Email    string `json:"email"    form:"email"`
	Password string `json:"password" form:"password"`
}

// UpdateProfileSchema validates UpdateProfile.
var UpdateProfileSchema = z.Struct[UpdateProfile](z.Shape{
	"name":     z.String().Trim().Min(2).Max(100),
	"email":    z.String().Trim().Email().Max(255),
	"password": z.String().Max(256).Optional(),
})

// ChangePassword is the password change form.
type ChangePassword struct {
	CurrentPassword      string `json:"current_password"      form:"current_password"`
	Password             string `json:"password"              form:"password"`
	PasswordConfirmation string `json:"password_confirmation" form:"password_confirmation"`
}

// ChangePasswordSchema validates ChangePassword (same rules as Register).
var ChangePasswordSchema = z.Struct[ChangePassword](z.Shape{
	"current_password":      z.String().Min(1).Max(256),
	"password":              z.String().Min(12).Max(256),
	"password_confirmation": z.String(),
}).Refine(func(r ChangePassword) error {
	if r.Password != r.PasswordConfirmation {
		return z.FieldError("password_confirmation", MsgPasswordMismatch)
	}

	return nil
})
