package component

import (
	"strings"
)

// Comps 组件注册
// Example:
//
//	Comps(NewComponent1(),NewComponent2(),NewComponent2())
func Comps(comps ...Component) *Components {
	components := &Components{}
	for i := 0; i < len(comps); i++ {
		options := make([]Option, 0)
		options = append(options, WithName(comps[i].Name()))
		if scheName := strings.TrimSpace(comps[i].SchedName()); len(scheName) > 0 {
			options = append(options, WithSchedulerName(comps[i].SchedName()))
		}
		components.Register(
			comps[i],
			options...,
		)
	}
	return components
}
