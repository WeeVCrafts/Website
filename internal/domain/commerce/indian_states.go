package commerce

import "strings"

func IndianStateCode(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	abbreviations := map[string]string{"jk": "01", "hp": "02", "pb": "03", "ch": "04", "uk": "05", "hr": "06", "dl": "07", "rj": "08", "up": "09", "br": "10", "sk": "11", "ar": "12", "nl": "13", "mn": "14", "mz": "15", "tr": "16", "ml": "17", "as": "18", "wb": "19", "jh": "20", "od": "21", "ct": "22", "mp": "23", "gj": "24", "dn": "26", "mh": "27", "ka": "29", "ga": "30", "ld": "31", "kl": "32", "tn": "33", "py": "34", "an": "35", "ts": "36", "ap": "37", "la": "38"}
	if code := abbreviations[s]; code != "" {
		return code
	}
	states := map[string]string{"jammu and kashmir": "01", "himachal pradesh": "02", "punjab": "03", "chandigarh": "04", "uttarakhand": "05", "haryana": "06", "delhi": "07", "rajasthan": "08", "uttar pradesh": "09", "bihar": "10", "sikkim": "11", "arunachal pradesh": "12", "nagaland": "13", "manipur": "14", "mizoram": "15", "tripura": "16", "meghalaya": "17", "assam": "18", "west bengal": "19", "jharkhand": "20", "odisha": "21", "chhattisgarh": "22", "madhya pradesh": "23", "gujarat": "24", "dadra and nagar haveli and daman and diu": "26", "maharashtra": "27", "karnataka": "29", "goa": "30", "lakshadweep": "31", "kerala": "32", "tamil nadu": "33", "puducherry": "34", "andaman and nicobar islands": "35", "telangana": "36", "andhra pradesh": "37", "ladakh": "38"}
	for _, code := range states {
		if s == code {
			return code
		}
	}
	return states[s]
}
func ValidIndianStateCode(code string) bool { return code != "" && IndianStateCode(code) == code }
