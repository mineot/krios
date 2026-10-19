package entity

import (
	"errors"
	"strings"
	"testing"

	"github.com/mineot/krios/internal/errs"
)

// mismatchUser is a struct of a different type than the entity under test,
// used to exercise the invalid-destination paths.
type mismatchUser struct {
	ID int64
}

type validationUser struct {
	ID    int64  `krios:"id"`
	Name  string `krios:"name"`
	Email email  `krios:"email"`
	Bio   *text  `krios:"bio"`
	Note  string `krios:"note"`
}

type email string

func (e email) Validate() error {
	if !strings.Contains(string(e), "@") {
		return errors.New("email must contain @")
	}
	return nil
}

func (e email) Format() (string, error) {
	return strings.ToUpper(string(e)), nil
}

type text string

// pointer receiver: exercises addressable fields and nil skipping.
func (t *text) Validate() error {
	if *t == "" {
		return errors.New("text must not be empty")
	}
	return nil
}

func (t *text) Format() (string, error) {
	return "<" + string(*t) + ">", nil
}

type badFormatter string

func (badFormatter) Format() (string, error) {
	return "", errors.New("cannot format")
}

type formattingErrUser struct {
	Value badFormatter `krios:"value"`
}

func TestEntityValidate(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	valid := validationUser{Name: "alice", Email: "a@b", Bio: newText("hi")}
	if err := ent.Validate(valid); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if err := ent.Validate(&valid); err != nil {
		t.Fatalf("Validate(*T) = %v", err)
	}

	invalidEmail := valid
	invalidEmail.Email = "nope"
	err = ent.Validate(invalidEmail)
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("Validate() = %v, want errs.ErrValidation", err)
	}
	if !strings.Contains(err.Error(), `column "email"`) || !strings.Contains(err.Error(), "validation_user") {
		t.Errorf("Validate() = %v, want table and column context", err)
	}
	if !strings.Contains(err.Error(), "must contain @") {
		t.Errorf("Validate() = %v, want preserved cause", err)
	}
}

func TestEntityValidateSkipsNilPointers(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	// Bio is nil and its validator would panic on a nil receiver; a nil
	// pointer must be skipped.
	valid := validationUser{Email: "a@b"}
	if err := ent.Validate(valid); err != nil {
		t.Fatalf("Validate() = %v, want nil for skipped nil pointer", err)
	}
}

func TestEntityValidateDetectsPointerReceiver(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	record := validationUser{Email: "a@b", Bio: newText("")}
	if err := ent.Validate(record); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("Validate() = %v, want errs.ErrValidation for empty *text", err)
	}
}

func TestEntityValidateReportsFirstField(t *testing.T) {
	type ordered struct {
		First  email `krios:"first"`
		Second email `krios:"second"`
	}
	ent, err := EntityOf[ordered]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	err = ent.Validate(ordered{First: "bad", Second: "also-bad"})
	if !strings.Contains(err.Error(), `column "first"`) {
		t.Errorf("Validate() = %v, want first field reported", err)
	}
}

func TestEntityValidateAll(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	valid := []validationUser{
		{Email: "a@b"},
		{Email: "c@d"},
	}
	if err := ent.ValidateAll(valid); err != nil {
		t.Fatalf("ValidateAll() = %v", err)
	}
	if err := ent.ValidateAll(&valid); err != nil {
		t.Fatalf("ValidateAll(*slice) = %v", err)
	}

	invalid := []validationUser{
		{Email: "a@b"},
		{Email: "nope"},
	}
	if err := ent.ValidateAll(invalid); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("ValidateAll() = %v, want errs.ErrValidation", err)
	}
}

func TestEntityValidateInvalidDestinations(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"nil validate", func() error { return ent.Validate(nil) }},
		{"wrong type", func() error { return ent.Validate(mismatchUser{}) }},
		{"nil pointer", func() error { var u *validationUser; return ent.Validate(u) }},
		{"nil slice", func() error { return ent.ValidateAll(nil) }},
		{"slice of wrong type", func() error { return ent.ValidateAll([]mismatchUser{}) }},
		{"pointer to value", func() error { return ent.ValidateAll(&validationUser{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, errs.ErrInvalidEntity) {
				t.Fatalf("call() = %v, want errs.ErrInvalidEntity", err)
			}
		})
	}
}

func TestEntityFormat(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	record := validationUser{ID: 7, Name: "alice", Email: "a@b", Bio: newText("hello"), Note: "plain"}
	got, err := ent.Format(record)
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}

	want := map[string]string{
		"id":    "7",
		"name":  "alice",
		"email": "A@B",
		"bio":   "<hello>",
		"note":  "plain",
	}
	for column, text := range want {
		if got[column] != text {
			t.Errorf("Format()[%q] = %q, want %q", column, got[column], text)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Format() returned %d columns, want %d", len(got), len(want))
	}
}

func TestEntityFormatNilPointerIsEmpty(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	got, err := ent.Format(validationUser{Email: "a@b"})
	if err != nil {
		t.Fatalf("Format() = %v", err)
	}
	if got["bio"] != "" {
		t.Errorf("Format()[bio] = %q, want empty", got["bio"])
	}
}

func TestEntityFormatError(t *testing.T) {
	ent, err := EntityOf[formattingErrUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	_, err = ent.Format(formattingErrUser{Value: "x"})
	if !errors.Is(err, errs.ErrFormat) {
		t.Fatalf("Format() = %v, want errs.ErrFormat", err)
	}
	if !strings.Contains(err.Error(), `column "value"`) || !strings.Contains(err.Error(), "formatting_err_user") {
		t.Errorf("Format() = %v, want table and column context", err)
	}
	if !strings.Contains(err.Error(), "cannot format") {
		t.Errorf("Format() = %v, want preserved cause", err)
	}
}

func TestEntityFormatInvalidDestination(t *testing.T) {
	ent, err := EntityOf[validationUser]()
	if err != nil {
		t.Fatalf("EntityOf() = %v", err)
	}

	if _, err := ent.Format("nope"); !errors.Is(err, errs.ErrInvalidEntity) {
		t.Fatalf("Format() = %v, want errs.ErrInvalidEntity", err)
	}
}

func newText(value string) *text {
	t := text(value)
	return &t
}
