package dataset

import (
	"strconv"
	"strings"
)

func NaturalLess(a, b string) bool {
	sa, sb := strings.TrimSpace(a), strings.TrimSpace(b)
	la, lb := strings.ToLower(sa), strings.ToLower(sb)
	for ia, ib := 0, 0; ia < len(la) || ib < len(lb); {
		if ia == len(la) || ib == len(lb) {
			return ia == len(la) && ib != len(lb)
		}
		if isDigit(la[ia]) && isDigit(lb[ib]) {
			ja, jb := ia, ib
			for ja < len(la) && isDigit(la[ja]) {
				ja++
			}
			for jb < len(lb) && isDigit(lb[jb]) {
				jb++
			}
			na, _ := strconv.ParseUint(la[ia:ja], 10, 64)
			nb, _ := strconv.ParseUint(lb[ib:jb], 10, 64)
			if na != nb {
				return na < nb
			}
			ia, ib = ja, jb
			continue
		}
		if la[ia] != lb[ib] {
			return la[ia] < lb[ib]
		}
		ia++
		ib++
	}
	return sa < sb
}

func isDigit(value byte) bool { return value >= '0' && value <= '9' }
