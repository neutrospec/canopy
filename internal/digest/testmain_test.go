package digest

import (
	"os"
	"testing"

	"github.com/neutrospec/canopy/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }
