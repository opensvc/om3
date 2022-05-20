package schedule

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	tmZero = "0001-01-01 Z00:00:00"
)

func TestTooSoon(t *testing.T) {
	tests := []struct {
		Expression string
		Time       string
		Last       string
	}{
		{"@10", "2015-02-27 Z10:00:00", "2015-02-27 Z09:52:00"},
		{"@10s", "2015-02-27 Z10:00:15", "2015-02-27 Z10:00:08"},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			sc := New(data.Expression)
			last, _ := time.Parse(timeLayout, data.Last)
			tm, _ := time.Parse(timeLayout, data.Time)
			_, err := sc.TestWithLast(tm, last)
			require.ErrorIs(t, err, ErrNotAllowed)
		})
	}
}

func TestNotAllowed(t *testing.T) {
	tests := []struct {
		Expression string
		Time       string
	}{
		{"", "2015-02-27 Z10:00:00"},
		{"@0", "2015-02-27 Z10:00:00"},
		{"*@0", "2015-02-27 Z10:00:00"},
		{"09:00-09:20", "2015-02-27 Z10:00:00"},
		{"09:00-09:20@31", "2015-02-27 Z10:00:00"},
		{"09:00-09:00", "2015-02-27 Z10:00:00"},
		{"09:00", "2015-02-27 Z10:00:00"},
		{"09:00-09:00", "2015-02-27 Z09:09:00"},
		{"09:00", "2015-02-27 Z09:09"},
		{"09:20-09:00", "2015-02-27 Z09:09:00"},
		{"* fri", "2015-10-08 Z10:00:00"},
		{"* *:-2", "2015-01-24 Z10:00:00"},
		{"* *:-2", "2015-01-31 Z10:00:00"},
		{"* :last", "2015-01-30 Z10:00:00"},
		{"* :-2", "2015-01-31 Z10:00:00"},
		{"* :-2", "2015-01-05 Z10:00:00"},
		{"* :5", "2015-01-06 Z10:00:00"},
		{"* :+5", "2015-01-06 Z10:00:00"},
		{"* :fifth", "2015-01-06 Z10:00"},
		{"* * * %2", "2015-01-06 Z10:00:00"},
		{"* * * jan-feb%2", "2015-01-06 Z10:00:00"},
	}

	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			sc := New(data.Expression)
			tm, _ := time.Parse(timeLayout, data.Time)
			_, err := sc.Test(tm)
			require.ErrorIs(t, err, ErrNotAllowed, "test data: %+v parsed schedule: %+v", data, sc.Dataset())
		})
	}
}

func TestAllowed(t *testing.T) {
	tests := []struct {
		Expression string
		Time       string
	}{
		{"*", "2015-02-27 Z10:00:00"},
		{"*@61", "2015-02-27 Z10:00:00"},
		{"09:20-09:00", "2015-02-27 Z10:00:00"},
		{"09:00-09:20", "2015-02-27 Z09:09:00"},
		{"~09:00-09:20", "2015-02-27 Z09:09:00"},
		{"09:00-09:20@31", "2015-02-27 Z09:09:00"},
		{"* fri", "2015-10-09 Z10:00:00"},

		{"* *:fifth", "2015-01-05 Z10:00:00"},
		{"* *:fourth", "2015-01-04 Z10:00:00"},
		{"* *:third", "2015-01-03 Z10:00:00"},
		{"* *:second", "2015-01-02 Z10:00:00"},
		{"* *:first", "2015-01-01 Z10:00:00"},
		{"* *:1st", "2015-01-01 Z10:00:00"},
		{"* *:last", "2015-01-31 Z10:00:00"},

		{"* :fifth", "2015-01-05 Z10:00:00"},
		{"* :fourth", "2015-01-04 Z10:00:00"},
		{"* :third", "2015-01-03 Z10:00:00"},
		{"* :second", "2015-01-02 Z10:00:00"},
		{"* :first", "2015-01-01 Z10:00:00"},
		{"* :1st", "2015-01-01 Z10:00:00"},
		{"* :last", "2015-01-31 Z10:00:00"},

		{"* :5", "2015-01-05 Z10:00:00"},
		{"* :4", "2015-01-04 Z10:00:00"},
		{"* :3", "2015-01-03 Z10:00:00"},
		{"* :2", "2015-01-02 Z10:00:00"},
		{"* :1", "2015-01-01 Z10:00:00"},
		{"* :-1", "2015-01-31 Z10:00:00"},
		{"* :-2", "2015-01-30 Z10:00:00"},
		{"* :-3", "2015-01-29 Z10:00:00"},
		{"* :-4", "2015-01-28 Z10:00:00"},

		{"* *:5", "2015-01-05 Z10:00:00"},
		{"* *:4", "2015-01-04 Z10:00:00"},
		{"* *:3", "2015-01-03 Z10:00:00"},
		{"* *:2", "2015-01-02 Z10:00:00"},
		{"* *:1", "2015-01-01 Z10:00:00"},
		{"* *:-1", "2015-01-31 Z10:00:00"},
		{"* *:-2", "2015-01-30 Z10:00:00"},
		{"* *:-3", "2015-01-29 Z10:00:00"},
		{"* *:-4", "2015-01-28 Z10:00:00"},

		{"* * * jan", "2015-01-06 Z10:00:00"},
		{"* * * jan-feb", "2015-01-06 Z10:00:00"},
		{"* * * %2+1", "2015-01-06 Z10:00:00"},
		{"* * * jan-feb%2+1", "2015-01-06 Z10:00:00"},
		{"18:00-18:59@60 wed", "2016-08-31 Z18:00:00"},
		{"23:00-23:59@61 *:first", "2016-09-01 Z23:00:00"},
		{"23:00-23:59", "2016-09-01 Z23:00:00"},
		{"23:00-00:59", "2016-09-01 Z23:00:00"},
		{"@10", "2015-02-27 Z10:00:00"},
	}

	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			sc := New(data.Expression)
			tm, _ := time.Parse(timeLayout, data.Time)
			_, err := sc.Test(tm)
			require.ErrorIs(t, err, nil)
		})
	}
}

func TestParsedInterval(t *testing.T) {
	tests := []struct {
		Expression string
		Duration   time.Duration
	}{
		{"@3s", time.Second * 3},
		{"*@6s", time.Second * 6},
		{"*@06s", time.Second * 6},
		{"*@18s", time.Second * 18},
		{"10:00-18:00@10s", time.Second * 10},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			parsed, err := parse(data.Expression)
			require.ErrorIs(t, err, nil)
			require.Len(t, parsed, 1)
			require.Len(t, parsed[0].Timeranges, 1)
			require.Equal(t, data.Duration, parsed[0].Timeranges[0].Interval)
		})
	}
}

func TestInvalidExpr(t *testing.T) {
	// non parsable expressions
	tests := []string{
		"23:00-23:59@61 *:first:*",
		"23:00-23:59@61 *:",
		"23:00-23:59@61 *:*",
		"23:00-23:59@61 * * %2%3",
		"23:00-23:59@61 * * %2+1+2",
		"23:00-23:59@61 * * %foo",
		"23:00-23:59@61 * * %2+foo",
		"23:00-23:59@61 freday",
		"23:00-23:59@61 * * junuary",
		"23:00-23:59@61 * * %2%3",
		"23:00-23:59-01:00@61",
		"23:00-23:59:00@61 * * %2%3",
		"23:00-23:59@61@10",
		"23:00-23:02 mon 1 12 4",
		"21-22 mon 1 12",
		"14-15",

		// :monthday 0, can't match anything
		"* :0",
		"* :0",
		"* :0",
		"* :0",
		"* *:0",
		"* *:0",
		"* *:0",
		"* *:0",
	}
	otherExpr := "09:00-09:20@60 :1st 1 january" // this one is parsable
	for _, test := range tests {
		expr := fmt.Sprintf("[\"%s\", \"%s\"]", test, otherExpr)
		t.Run(expr, func(t *testing.T) {
			_, err := parse(expr)
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func TestParseWeeks(t *testing.T) {
	tests := []struct {
		Expression string
		Parsed     []int
	}{
		{"1-5", []int{1, 2, 3, 4, 5}},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			parsed, err := parseWeeks(data.Expression)
			require.Nil(t, err)
			require.Equal(t, data.Parsed, parsed)
		})
	}
}

func TestParseMonths(t *testing.T) {
	tests := []struct {
		Expression string
		Parsed     []int
	}{
		{"dec-mar", []int{1, 2, 3, 12}},
		{"april-apr", []int{4}},
		{"jun", []int{6}},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			parsed, err := parseMonths(data.Expression)
			require.Nil(t, err)
			require.Equal(t, data.Parsed, parsed)
		})
	}
}

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		Expression string
		Parsed     []int
	}{
		{"mon", []int{1}},
		{"monday", []int{1}},
		{"sun", []int{7}},
		{"sunday", []int{7}},
		{"monday-sunday", []int{1, 2, 3, 4, 5, 6, 7}},
		{"monday-wed", []int{1, 2, 3}},
		{"sun-mon", []int{1, 7}},
		{"sun-tue", []int{1, 2, 7}},
		{"sun-wed", []int{1, 2, 3, 7}},
		{"mon-monday", []int{1}},
		{"tuesday-tue", []int{2}},
		{"sun-fri", []int{1, 2, 3, 4, 5, 7}},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			parsed, err := parseWeekdays(data.Expression)
			require.Nil(t, err)
			require.Equal(t, data.Parsed, parsed)
		})
	}
}

func TestFilterWithModulo(t *testing.T) {
	tests := []struct {
		Expression string
		Original   []int
		Filtered   []int
	}{
		{"2", []int{1, 2, 3, 4, 5, 6, 7, 8}, []int{2, 4, 6, 8}},
		{"2+1", []int{1, 2, 3, 4, 5, 6, 7, 8}, []int{1, 3, 5, 7}},
		{"3", []int{1, 2, 3, 4, 5, 6, 7, 8}, []int{3, 6}},
		{"3+1", []int{1, 2, 3, 4, 5, 6, 7, 8}, []int{2, 5, 8}},
		{"3+2", []int{1, 2, 3, 4, 5, 6, 7, 8}, []int{1, 4, 7}},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			filtered, err := filterWithModulo(data.Original, data.Expression)
			require.Nil(t, err)
			require.Equal(t, data.Filtered, filtered)
		})
	}
}

func TestMonthDays(t *testing.T) {
	tests := []struct {
		Time string
		Days int
	}{
		{"2022-02-02 Z00:00:00", 28},
		{"2022-03-02 Z00:00:00", 31},
		{"2022-04-02 Z00:00:00", 30},
		{"2023-02-02 Z00:00:00", 28},
		{"2024-02-02 Z00:00:00", 29},
	}
	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			tm, err := time.Parse(timeLayout, data.Time)
			require.ErrorIs(t, err, nil)
			days := monthDays(tm)
			require.Equal(t, data.Days, days)
		})
	}
}

func TestNext(t *testing.T) {
	tests := []struct {
		Expression string
		Time       string
		Last       string
		Next       string
	}{
		{"09:00-09:20", "2015-02-27 Z10:00:00", "2015-02-27 Z09:05:00", "2015-02-28 Z09:00:00"},
		{"09:00-09:20 mon", "2022-05-20 Z10:00:00", "2022-05-16 Z09:05:00", "2022-05-23 Z09:00:00"},
		{"09:00-09:20", "2015-02-28 Z10:00:00", "2015-02-27 Z09:05:00", "2015-03-01 Z09:00:00"},
		{"", "2015-02-27 Z10:00:00", "2015-02-27 Z08:00:00", tmZero},
		{"@0", "2015-02-27 Z10:00:00", "2015-02-27 Z08:00:00", tmZero},
		{"*@0", "2015-02-27 Z10:00:00", "2015-02-27 Z08:00:00", tmZero},
		/*
			{"@0", "2015-02-27 Z10:00:00"},
			{"*@0", "2015-02-27 Z10:00:00"},
			{"09:00-09:20", "2015-02-27 Z10:00:00"},
			{"09:00-09:20@31", "2015-02-27 Z10:00:00"},
			{"09:00-09:00", "2015-02-27 Z10:00:00"},
			{"09:00", "2015-02-27 Z10:00:00"},
			{"09:00-09:00", "2015-02-27 Z09:09:00"},
			{"09:00", "2015-02-27 Z09:09"},
			{"09:20-09:00", "2015-02-27 Z09:09:00"},
			{"* fri", "2015-10-08 Z10:00:00"},
			{"* *:-2", "2015-01-24 Z10:00:00"},
			{"* *:-2", "2015-01-31 Z10:00:00"},
			{"* :last", "2015-01-30 Z10:00:00"},
			{"* :-2", "2015-01-31 Z10:00:00"},
			{"* :-2", "2015-01-05 Z10:00:00"},
			{"* :5", "2015-01-06 Z10:00:00"},
			{"* :+5", "2015-01-06 Z10:00:00"},
			{"* :fifth", "2015-01-06 Z10:00"},
			{"* * * %2", "2015-01-06 Z10:00:00"},
			{"* * * jan-feb%2", "2015-01-06 Z10:00:00"},
		*/
	}

	for _, data := range tests {
		name := fmt.Sprintf("%+v", data)
		t.Run(name, func(t *testing.T) {
			sc := New(data.Expression)
			tm, err := time.Parse(timeLayout, data.Time)
			require.ErrorIs(t, err, nil)
			last, err := time.Parse(timeLayout, data.Last)
			require.ErrorIs(t, err, nil)
			expectedNext, err := time.Parse(timeLayout, data.Next)
			require.ErrorIs(t, err, nil)
			next, _, err := sc.Next(NextWithTime(tm), NextWithLast(last))
			require.ErrorIs(t, err, nil)
			require.Equal(t, expectedNext, next)
		})
	}
}
