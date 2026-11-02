package admin

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func userManagementActive(p viewmodels.AdminPage) bool {
	if !p.CanSecurity {
		return false
	}
	switch p.Workspace {
	case "users", "roles", "security", "security-events", "audit-logs", "settings", "system-health", "search-indexing-health", "worker-queue-health":
		return true
	}
	return p.Active == "security"
}
func userManagementNavKey(p viewmodels.AdminPage) string {
	if userManagementActive(p) {
		return "user-management"
	}
	return p.NavActive
}

func userManagementSection(p viewmodels.AdminPage) string {
	switch p.Workspace {
	case "users":
		return "Accounts"
	case "roles":
		return "Roles"
	case "settings", "system-health", "search-indexing-health", "worker-queue-health":
		return "Settings"
	default:
		return "Security"
	}
}

func superAdminArt(p viewmodels.AdminPage) string {
	if userManagementActive(p) {
		return "access"
	}
	if p.Active == "support" {
		return "support"
	}
	switch p.Active {
	case "dashboard", "orders", "finance":
		return "header"
	default:
		return "package"
	}
}

func superAdminExportURL(p viewmodels.AdminPage, kind string) string {
	values := url.Values{"q": {p.Query}, "range": {strconv.Itoa(p.Insights.Days)}}
	if p.StatusFilter != "" {
		values.Set("status", p.StatusFilter)
	}
	return p.PortalRoot + "/" + kind + "/export?" + values.Encode()
}

func chartMaximum(points []viewmodels.AdminChartPoint) int64 {
	var max int64 = 1
	for _, point := range points {
		if point.Value > max {
			max = point.Value
		}
	}
	return max
}
func chartPoints(points []viewmodels.AdminChartPoint) string {
	var result []string
	for i, point := range points {
		x := float64(50)
		if len(points) > 1 {
			x += float64(i) * 540 / float64(len(points)-1)
		}
		y := 190 - float64(point.Value)*155/float64(chartMaximum(points))
		result = append(result, fmt.Sprintf("%.2f,%.2f", x, y))
	}
	return strings.Join(result, " ")
}
func chartArea(points []viewmodels.AdminChartPoint) string {
	if len(points) == 0 {
		return ""
	}
	return "50,190 " + chartPoints(points) + " 590,190"
}
func chartValue(value int64, currency bool) string {
	if currency {
		return fmt.Sprintf("₹%.0f", float64(value)/100)
	}
	return strconv.FormatInt(value, 10)
}

func refundBarX(index, count int) string {
	return fmt.Sprintf("%.2f", float64(index)*570/float64(max(count, 1))+10)
}
func refundBarWidth(count int) string {
	return fmt.Sprintf("%.2f", max(2.0, 570/float64(max(count, 1))-4))
}
func refundBarHeight(points []viewmodels.AdminChartPoint, index int) string {
	return fmt.Sprintf("%.2f", float64(points[index].Value)*90/float64(chartMaximum(points)))
}
func refundBarY(points []viewmodels.AdminChartPoint, index int) string {
	return fmt.Sprintf("%.2f", 100-float64(points[index].Value)*90/float64(chartMaximum(points)))
}
func chartTotal(slices []viewmodels.AdminChartSlice) int64 {
	var total int64
	for _, slice := range slices {
		total += slice.Value
	}
	return total
}
func sliceDash(slices []viewmodels.AdminChartSlice, i int) string {
	total := chartTotal(slices)
	if total == 0 {
		return "0 100"
	}
	part := 100 * float64(slices[i].Value) / float64(total)
	return fmt.Sprintf("%.3f %.3f", part, 100-part)
}
func sliceOffset(slices []viewmodels.AdminChartSlice, i int) string {
	var before int64
	for _, slice := range slices[:i] {
		before += slice.Value
	}
	if chartTotal(slices) == 0 {
		return "0"
	}
	return fmt.Sprintf("%.3f", -100*float64(before)/float64(chartTotal(slices)))
}
func slicePercent(slices []viewmodels.AdminChartSlice, i int) string {
	if chartTotal(slices) == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(slices[i].Value)/float64(chartTotal(slices)))
}

func financeSectionURL(p viewmodels.AdminPage, path string) string {
	base := p.PortalRoot + "/" + path
	if !p.CanSecurity {
		return base
	}
	values := url.Values{"range": {strconv.Itoa(p.Insights.Days)}}
	if p.Query != "" {
		values.Set("q", p.Query)
	}
	return base + "?" + values.Encode()
}

func statIsMoney(value string) bool { return strings.HasPrefix(value, "INR ") }
