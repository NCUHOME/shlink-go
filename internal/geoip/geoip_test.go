package geoip

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocatorWithoutDatabaseReturnsEmptyLocation(t *testing.T) {
	locator, err := Open("")
	require.NoError(t, err)
	require.NotNil(t, locator)
	require.Nil(t, locator.Lookup(net.ParseIP("1.2.3.4")))
	require.NoError(t, locator.Close())
}

func TestLocatorHandlesNilInput(t *testing.T) {
	locator, err := Open("")
	require.NoError(t, err)
	require.Nil(t, locator.Lookup(nil))
}
