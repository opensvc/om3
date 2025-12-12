package clusterip

import (
	"net"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
)

func TestSort(t *testing.T) {
	sortedList := func() L {
		return L{
			{IP: net.IP{10, 8, 0, 8}, Node: "test", Path: naming.Path{Name: "a", Namespace: "A", Kind: naming.KindCcfg}, RID: "ip#10"},
			{IP: net.IP{10, 200, 3, 1}, Node: "test", Path: naming.Path{Name: "b", Namespace: "A", Kind: naming.KindSvc}, RID: "ip#3"},
			{IP: net.IP{10, 6, 6, 9}, Node: "a", Path: naming.Path{Name: "z", Namespace: "C", Kind: naming.KindVol}, RID: "ip#99"},
			{IP: net.IP{10, 0, 0, 0}, Node: "b", Path: naming.Path{Name: "z", Namespace: "C", Kind: naming.KindVol}, RID: "ip#70"},
			{IP: net.IP{10, 20, 1, 1}, Node: "b", Path: naming.Path{Name: "z", Namespace: "D", Kind: naming.KindSvc}, RID: "ip#10"},
			{IP: net.IP{10, 30, 0, 1}, Node: "b", Path: naming.Path{Name: "z", Namespace: "D", Kind: naming.KindSvc}, RID: "ip#10"},
			{IP: net.IP{10, 99, 99, 1}, Node: "b", Path: naming.Path{Name: "z", Namespace: "D", Kind: naming.KindUsr}, RID: "ip#10"},
			{IP: net.IP{10, 0, 99, 1}, Node: "b", Path: naming.Path{Name: "z", Namespace: "D", Kind: naming.KindUsr}, RID: "ip#8"},
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
