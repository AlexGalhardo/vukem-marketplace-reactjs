package requests

import "github.com/AlexGalhardo/vukem/z"

// Login is the login form.
type Login struct {
	Email    string `json:"email"    form:"email"`
	Password string `json:"password" form:"password"`
	Remember bool   `json:"remember" form:"remember"`
}

// LoginSchema validates Login.
var LoginSchema = z.Struct[Login](z.Shape{
	"email":    z.String().Trim().Email(),
	"password": z.String().Min(1),
	"remember": z.Bool().Optional(),
})

// Register is the registration form.
type Register struct {
	Name                 string `json:"name"                  form:"name"`
	Email                string `json:"email"                 form:"email"`
	Password             string `json:"password"              form:"password"`
	PasswordConfirmation string `json:"password_confirmation" form:"password_confirmation"`
}

// RegisterSchema validates Register (password rules live here).
var RegisterSchema = z.Struct[Register](z.Shape{
	"name":                  NameSchema(),
	"email":                 z.String().Trim().Email().Max(255),
	"password":              z.String().Min(12).Max(256),
	"password_confirmation": z.String(),
}).Refine(func(r Register) error {
	if r.Password != r.PasswordConfirmation {
		return z.FieldError("password_confirmation", MsgPasswordMismatch)
	}

	return nil
})

// ForgotPassword is the reset link request form.
type ForgotPassword struct {
	Email string `json:"email" form:"email"`
}

// ForgotPasswordSchema validates ForgotPassword.
var ForgotPasswordSchema = z.Struct[ForgotPassword](z.Shape{"email": z.String().Trim().Email()})

// ResetPassword is the new password form.
type ResetPassword struct {
	Token                string `json:"token"                 form:"token"`
	Email                string `json:"email"                 form:"email"`
	Password             string `json:"password"              form:"password"`
	PasswordConfirmation string `json:"password_confirmation" form:"password_confirmation"`
}

// ResetPasswordSchema validates ResetPassword.
var ResetPasswordSchema = z.Struct[ResetPassword](z.Shape{
	"token":                 z.String().Min(1),
	"email":                 z.String().Trim().Email(),
	"password":              z.String().Min(12).Max(256),
	"password_confirmation": z.String(),
}).Refine(func(r ResetPassword) error {
	if r.Password != r.PasswordConfirmation {
		return z.FieldError("password_confirmation", MsgPasswordMismatch)
	}

	return nil
})
