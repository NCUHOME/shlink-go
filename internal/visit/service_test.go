package visit

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnonymizeIP(t *testing.T) {
	require.Equal(t, "1.2.3.0", anonymizeIP(net.ParseIP("1.2.3.4")).String())
	require.Equal(t, "2001:db8:1234:5678::", anonymizeIP(net.ParseIP("2001:db8:1234:5678:9abc:def0:1234:5678")).String())
}

func TestNormalizeIP(t *testing.T) {
	require.Equal(t, "1.2.3.4", normalizeIP("1.2.3.4").String())
	require.Equal(t, "1.2.3.4", normalizeIP("1.2.3.4:8080").String())
	require.Nil(t, normalizeIP(""))
	require.Nil(t, normalizeIP("not an ip"))
}
