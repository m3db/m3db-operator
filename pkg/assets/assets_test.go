package assets

import (
	"io/ioutil"
	"testing"

	"github.com/rakyll/statik/fs"
	"github.com/stretchr/testify/require"
)

func TestAssetsGenerated(t *testing.T) {
	hfs, err := fs.New()
	require.NoError(t, err)

	packaged, err := fs.ReadFile(hfs, "/default-config.tmpl")
	require.NoError(t, err)

	raw, err := ioutil.ReadFile("../../assets/default-config.tmpl")
	require.NoError(t, err)

	require.Equal(t, string(raw), string(packaged))
}
