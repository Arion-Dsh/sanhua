package sanhua

import (
	"sort"
	"testing"
)

func TestConn(t *testing.T) {

	t.Run("sequenceGreaterThan", func(t *testing.T) {
		s := []uint32{100, 144, 15, 133, 132}
		d := make([]uint32, 5)
		copy(d[1:], s)
		sort.Slice(d, func(i, j int) bool {
			return sequenceGreaterThan(d[i], d[j])
		})
		for i, v := range []uint32{144, 133, 100, 15, 0} {
			if d[i] != v {
				t.Fatal("sequenceGreaterThan err")
			}

		}

	})

}
