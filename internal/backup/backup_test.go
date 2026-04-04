package backup

import (
	"testing"
	"time"
)

func TestGetTimeData(t *testing.T) {
	timeData := GetTimeData()

	// Test that all fields are populated
	if timeData.DateTime == "" {
		t.Fatal("DateTime should not be empty")
	}

	if timeData.DayOfWeek == "" {
		t.Fatal("DayOfWeek should not be empty")
	}

	if timeData.Month == "" {
		t.Fatal("Month should not be empty")
	}

	// Test ISO weekday conversion (1=Monday, 7=Sunday)
	if timeData.DayNumberOfWeek < 1 || timeData.DayNumberOfWeek > 7 {
		t.Fatalf("DayNumberOfWeek should be 1-7, got %d", timeData.DayNumberOfWeek)
	}

	// Test day of month (1-31)
	if timeData.DayOfMonth < 1 || timeData.DayOfMonth > 31 {
		t.Fatalf("DayOfMonth should be 1-31, got %d", timeData.DayOfMonth)
	}

	// Test week number (1-53)
	if timeData.WeekNumber < 1 || timeData.WeekNumber > 53 {
		t.Fatalf("WeekNumber should be 1-53, got %d", timeData.WeekNumber)
	}
}

func TestGetWeekNumber(t *testing.T) {
	// Test with a known date
	testDate := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC) // Thursday, June 15, 2023
	weekNum := getWeekNumber(testDate)

	// This should be week 24 in 2023
	if weekNum != 24 {
		t.Fatalf("Expected week 24, got %d", weekNum)
	}
}

func TestCalculateRemWeek(t *testing.T) {
	tests := []struct {
		weekNumber    int
		keepWeeklyNum int
		expected      string
	}{
		{5, 4, "01"},  // 5 - 4 = 1, with leading zero
		{10, 4, "06"}, // 10 - 4 = 6, with leading zero
		{15, 4, "11"}, // 15 - 4 = 11, no leading zero needed
		{2, 4, "51"},  // 53 - 4 + 2 = 51 (wraparound case)
	}

	for _, test := range tests {
		result := calculateRemWeek(test.weekNumber, test.keepWeeklyNum)
		if result != test.expected {
			t.Fatalf("calculateRemWeek(%d, %d) = %q, expected %q",
				test.weekNumber, test.keepWeeklyNum, result, test.expected)
		}
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		dbType   string
		expected string
	}{
		{"postgres", "dump"},
		{"mariadb", "sql"},
		{"mysql", "sql"},
		{"redis", "rdb"},
		{"mongo", "archive"},
		{"mongodb", "archive"},
		{"unknown", "dump"}, // default case
	}

	for _, test := range tests {
		result := getFileExtension(test.dbType)
		if result != test.expected {
			t.Fatalf("getFileExtension(%q) = %q, expected %q",
				test.dbType, result, test.expected)
		}
	}
}
