package mailsender

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapErr(t *testing.T) {
	{
		apperr := WrapErr(http.StatusInternalServerError, errors.New("wrap"))
		require.NotNil(t, apperr)
		require.Equal(t, http.StatusInternalServerError, apperr.Code)
		require.Equal(t, "wrap", apperr.Message)
		require.Equal(t, errors.New("wrap"), apperr.Internal)
	}

	{
		apperr := WrapErr(http.StatusInternalServerError, nil)
		require.Nil(t, apperr)
	}
}

func TestAppErr(t *testing.T) {
	apperr := AppErr(http.StatusInternalServerError, "error")
	require.NotNil(t, apperr)
	require.Equal(t, "error", apperr.Error())
}

func Test_appendError(t *testing.T) {
	{
		err := appendError(nil, nil)
		require.Nil(t, err)
	}

	{
		err := appendError(errors.New("error1"), nil)
		require.Equal(t, errors.New("error1"), err)
	}

	{
		err := appendError(nil, errors.New("error2"))
		require.Equal(t, errors.New("error2"), err)
	}

	{
		err := appendError(errors.New("error1"), errors.New("error2"))
		require.Equal(t, "error1\nerror2", err.Error())
	}
}
