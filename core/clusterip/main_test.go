package clusterip

import (
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/path"
	"github.com/stretchr/testify/assert"
	"net"
	"sort"
	"testing"
)

func TestSort(t *testing.T) {
	sortedList := func() L {
		return L{
			{IP: net.IP{10, 8, 0, 8}, Node: "test", Path: path.T{Name: "a", Namespace: "A", Kind: kind.T(64)}, RID: "ip#10"},
			{IP: net.IP{10, 200, 3, 1}, Node: "test", Path: path.T{Name: "b", Namespace: "A", Kind: kind.T(2)}, RID: "ip#3"},
			{IP: net.IP{10, 6, 6, 9}, Node: "a", Path: path.T{Name: "z", Namespace: "C", Kind: kind.T(4)}, RID: "ip#99"},
			{IP: net.IP{10, 0, 0, 0}, Node: "b", Path: path.T{Name: "z", Namespace: "C", Kind: kind.T(4)}, RID: "ip#70"},
			{IP: net.IP{10, 20, 1, 1}, Node: "b", Path: path.T{Name: "z", Namespace: "D", Kind: kind.T(2)}, RID: "ip#10"},
			{IP: net.IP{10, 0, 0, 1}, Node: "b", Path: path.T{Name: "z", Namespace: "D", Kind: kind.T(2)}, RID: "ip#10"},
			{IP: net.IP{10, 99, 99, 1}, Node: "b", Path: path.T{Name: "z", Namespace: "D", Kind: kind.T(32)}, RID: "ip#10"},
			{IP: net.IP{10, 0, 99, 1}, Node: "b", Path: path.T{Name: "z", Namespace: "D", Kind: kind.T(32)}, RID: "ip#8"},
		}
	}
	unsortedList := func(order []int) L {
		list := L{}
		ori := sortedList()
		for _, v := range order {
			list = append(list, ori[v])
		}
		return list
	}
	listToBeSorted := unsortedList([]int{1, 2, 0, 3, 6, 4, 5, 7})
	sort.Sort(listToBeSorted)
	assert.Equal(t, sortedList(), listToBeSorted)

	listToBeSorted = unsortedList([]int{4, 5, 6, 1, 3, 2, 0, 7})
	sort.Sort(listToBeSorted)
	assert.Equal(t, sortedList(), listToBeSorted)

	listToBeSorted = unsortedList([]int{4, 0, 5, 2, 1, 6, 7, 3})
	sort.Sort(listToBeSorted)
	assert.Equal(t, sortedList(), listToBeSorted)

	listToBeSorted = unsortedList([]int{6, 7, 3, 4, 5, 2, 1, 0})
	sort.Sort(listToBeSorted)
	assert.Equal(t, sortedList(), listToBeSorted)

}
