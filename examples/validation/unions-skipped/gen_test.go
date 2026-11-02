package gen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

// The package compiles without the validator, and a constraint on a union variant is not enforced.

func TestSkippedValidation_TwoVariants(t *testing.T) {
	// minimum is 0, but nothing checks it
	buffer := Box_Buffer_AnyOf{Either: runtime.NewEitherFromB[Percent, int](-1)}
	assert.NoError(t, buffer.Validate())

	buffer = Box_Buffer_AnyOf{Either: runtime.NewEitherFromA[Percent, int](Percent{})}
	assert.NoError(t, buffer.Validate())
}

func TestSkippedValidation_MoreThanTwoVariants(t *testing.T) {
	var size Crate_Size_OneOf

	// minimum is 1, but nothing checks it
	require.NoError(t, size.FromInt(0))
	n, err := size.AsValidatedInt()
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// minLength is 3, but nothing checks it
	require.NoError(t, size.FromString("a"))
	s, err := size.AsValidatedString()
	require.NoError(t, err)
	assert.Equal(t, "a", s)
}

func TestSkippedValidation_IfThenElse(t *testing.T) {
	// minLength is 3, but nothing checks it
	then := Parcel_IfThenElse{Conditional: runtime.NewConditionalFromThen[string, int]("a")}
	assert.NoError(t, then.Validate())

	// minimum is 1, but nothing checks it
	other := Parcel_IfThenElse{Conditional: runtime.NewConditionalFromElse[string, int](0)}
	assert.NoError(t, other.Validate())
}
