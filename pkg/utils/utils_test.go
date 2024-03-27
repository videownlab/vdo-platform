package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandAlphanumeric(t *testing.T) {
	{
		an := RandAlphanumeric(6)
		assert.Equal(t, 6, len(an))
		fmt.Println(an)
	}

	{
		an := RandAlphanumeric(4)
		assert.Equal(t, 4, len(an))
		fmt.Println(an)
	}
}
