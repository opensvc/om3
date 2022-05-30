package schedule

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	errchain "github.com/g8rswimmer/error-chain"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/seq"
)

var (
	timeLayout = "2006-01-02 Z15:04:05"

	ErrNotAllowed = errors.New("not allowed")
	ErrExcluded   = errors.New("excluded")
	ErrInvalid    = errors.New("invalid expression")
	ErrImpossible = errors.New("impossible schedule")
	ErrDrift      = errors.New("drift")
	ErrNextDay    = errors.New("next day")

	// ISO-8601 weeks. week one is the first week with thursday in year

	SchedFmt    = "%s: %s"
	AllMonths   = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	AllWeekdays = []int{1, 2, 3, 4, 5, 6, 7}
	AllDays     = []Day{
		{Weekday: 1},
		{Weekday: 2},
		{Weekday: 3},
		{Weekday: 4},
		{Weekday: 5},
		{Weekday: 6},
		{Weekday: 7},
	}
	CalendarNames = map[string]int{
		"jan":       1,
		"feb":       2,
		"mar":       3,
		"apr":       4,
		"may":       5,
		"jun":       6,
		"jul":       7,
		"aug":       8,
		"sep":       9,
		"oct":       10,
		"nov":       11,
		"dec":       12,
		"january":   1,
		"february":  2,
		"march":     3,
		"april":     4,
		"june":      6,
		"july":      7,
		"august":    8,
		"september": 9,
		"october":   10,
		"november":  11,
		"december":  12,
		"mon":       1,
		"tue":       2,
		"wed":       3,
		"thu":       4,
		"fri":       5,
		"sat":       6,
		"sun":       7,
		"monday":    1,
		"tuesday":   2,
		"wednesday": 3,
		"thursday":  4,
		"friday":    5,
		"saturday":  6,
		"sunday":    7,
	}
)

type (
	direction int
	Timerange struct {
		Probabilistic bool
		Interval      time.Duration
		Begin         time.Duration // since 00:00:00
		End           time.Duration // since 00:00:00
	}
	Timeranges []Timerange
	Day        struct {
		Weekday  int
		Monthday int
	}
	ExprData struct {
		Timeranges Timeranges
		Days       []Day
		Weeks      []int
		Months     []int
		Raw        string
		Exclude    bool
	}
	ExprDataset []ExprData
	Expr        struct {
		raw     string
		dataset ExprDataset
	}
)

func New(s string) *Expr {
	expr := &Expr{
		raw: s,
	}
	return expr
}

func (t *Expr) Append(s string) error {
	t.raw = strings.Join([]string{t.raw, s}, " ")
	return t.makeDataset()
}

func (t *Expr) AppendExprDataset(ds ExprDataset) {
	for _, data := range ds {
		t.raw = strings.Join([]string{t.raw, data.Raw}, " ")
		t.dataset = append(t.dataset, data)
	}
}

func (t Expr) String() string {
	return t.raw
}

func (t *Expr) Dataset() ExprDataset {
	_ = t.makeDataset()
	return t.dataset
}

func (t *Expr) makeDataset() error {
	if t.dataset != nil {
		return nil
	}
	if dataset, err := parse(t.raw); err != nil {
		return err
	} else {
		t.dataset = dataset
	}
	return nil
}

// ISOWeekday is like (time.Time).Weekday, but sunday is 7 instead of 0
func ISOWeekday(tm time.Time) int {
	i := int(tm.Weekday())
	switch i {
	case 0:
		return 7
	default:
		return i
	}
}

// monthDays returns the number of days in the month including tm
func monthDays(tm time.Time) int {
	firstDay := time.Date(tm.Year(), tm.Month(), 1, 0, 0, 0, 0, tm.Location())
	lastDay := firstDay.AddDate(0, 1, -1)
	return lastDay.Day()
}

// ContextualizeDays returns a copy of Days with the "nth-from-tail" monthdays
// evaluated using the actual number of days in the <tm> month
func (t ExprData) ContextualizeDays(tm time.Time) []Day {
	max := monthDays(tm)
	days := make([]Day, len(t.Days))
	for i, day := range t.Days {
		days[i] = day
		if day.Monthday == 0 {
			continue
		}
		if day.Monthday > 0 {
			continue
		}
		if -day.Monthday > max {
			continue
		}
		days[i].Monthday = max + day.Monthday + 1
	}
	return days
}

// isTooSoon return true if tm is before last+interval
func isTooSoon(tm, last time.Time, interval time.Duration) bool {
	if last.IsZero() {
		return false
	}
	if tm.Sub(last) >= interval {
		return false
	}
	return true
}

// After returns true if Begin > <tm>
func (t Timerange) After(tm time.Time) bool {
	begin := t.Begin
	seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
	return begin >= seconds
}

// TestIncludes returns ErrNotAllowed if <tm> is not in the Timerange
func (t Timerange) TestIncludes(tm time.Time) error {
	begin := t.Begin
	end := t.End
	seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))

	switch {
	case begin <= end:
		if (seconds >= begin) && (seconds <= end) {
			return nil
		}
	case begin > end:
		//
		//     =================
		//     23h     0h      1h
		//
		switch {
		case ((seconds >= begin) && (seconds <= time.Hour*24)):
			return nil
		case ((seconds >= 0) && (seconds <= end)):
			return nil
		}
	}
	return errors.Wrapf(ErrNotAllowed, "not in timerange %s-%s", t.Begin, t.End)
}

// TestIncludes returns true if <tm> is in the Timerange
func (t Timerange) Includes(tm time.Time) bool {
	err := t.TestIncludes(tm)
	return err == nil
}

//
// Delay returns a delay in seconds, compatible with the timerange.
//
// The daemon scheduler thread will honor this delay,
// executing the task only when expired.
//
// This algo is meant to level collector's load which peaks
// when tasks trigger at the same second on every nodes.
//
func (tr Timerange) Delay(tm time.Time) time.Duration {
	if !tr.Probabilistic {
		return 0
	}
	begin := tr.Begin
	end := tr.End
	seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
	if tr.Begin > tr.End {
		end += time.Hour * 24
	}
	if seconds < begin {
		seconds += time.Hour * 24
	}
	length := end - begin
	remaining := end - seconds - 1*time.Second

	if remaining < 1 {
		// no need to delay for tasks with a short remaining valid time
		return 0
	}

	if tr.Interval < length {
		// don't delay if interval < period length, because the user
		// expects the action to run multiple times in the period. And
		// '@<n>' interval-only schedule are already different across
		// nodes due to daemons not starting at the same moment.
		return 0
	}

	return time.Duration(float64(remaining) * rand.Float64())
}

func (t Timeranges) Including(tm time.Time) Timeranges {
	trs := make(Timeranges, 0)
	for _, tr := range t {
		if !tr.Includes(tm) {
			continue
		}
		trs = append(trs, tr)
	}
	return trs
}

func (t Timeranges) After(tm time.Time) Timeranges {
	trs := make(Timeranges, 0)
	for _, tr := range t {
		if !tr.After(tm) {
			continue
		}
		trs = append(trs, tr)
	}
	return trs
}

func (t Timeranges) SortedByIntervalAndBegin() Timeranges {
	trs := make(Timeranges, len(t))
	for i, tr := range t {
		trs[i] = tr
	}
	sort.Slice(trs, func(i, j int) bool {
		switch {
		case trs[i].Interval < trs[j].Interval:
			return true
		case trs[i].Interval == trs[j].Interval:
			return trs[i].Interval < trs[j].Interval
		default:
			return false
		}
	})
	return trs
}

func (t ExprData) GetTimerange(tm, last time.Time) (time.Time, time.Duration, error) {
	// if the candidate date is inside timeranges, return (candidate, smallest interval)
	trs := t.Timeranges.Including(tm).SortedByIntervalAndBegin()
	for _, tr := range trs {
		if isTooSoon(tm, last, tr.Interval) {
			tmi := last.Add(tr.Interval)
			if tr.Includes(tmi) {
				return tmi, tr.Interval, nil
			}
			return tm, 0, ErrNotAllowed
		}
		if tr.Probabilistic {
			delay := tr.Delay(tm)
			tmi := tm.Add(delay)
			return tmi, tr.Interval - delay, nil
		}
		return tm, tr.Interval, nil
	}

	// the candidate date is outside timeranges, return the closest range's (begin, interval)
	trs = t.Timeranges.After(tm).SortedByIntervalAndBegin()
	for _, tr := range trs {
		tm := time.Date(
			tm.Year(), tm.Month(), tm.Day(),
			0, 0, 0, 0,
			tm.Location(),
		).Add(tr.Begin)
		if isTooSoon(tm, last, tr.Interval) {
			tmi := last.Add(tr.Interval)
			if tr.Includes(tmi) {
				return tmi, tr.Interval, nil
			}
			continue
		}
		if tr.Probabilistic {
			delay := tr.Delay(tm)
			tmi := tm.Add(delay)
			return tmi, tr.Interval - delay, nil
		}
		return tm, tr.Interval, nil
	}

	return tm, 0, ErrNotAllowed
}

func (t ExprData) IsInWeeks(tm time.Time) bool {
	err := t.TestIsInWeeks(tm)
	return err == nil
}

func (t ExprData) TestIsInWeeks(tm time.Time) error {
	_, ref := tm.ISOWeek()
	for _, week := range t.Weeks {
		if week == ref {
			return nil
		}
	}
	return errors.Wrap(ErrNotAllowed, "not in allowed weeks")
}

func (t ExprData) IsInMonths(tm time.Time) bool {
	err := t.TestIsInMonths(tm)
	return err == nil
}

func (t ExprData) TestIsInMonths(tm time.Time) error {
	ref := int(tm.Month())
	for _, month := range t.Months {
		if month == ref {
			return nil
		}
	}
	return errors.Wrap(ErrNotAllowed, "not in allowed months")
}

func (t ExprData) IsInDays(tm time.Time) bool {
	err := t.TestIsInDays(tm)
	return err == nil
}

func (t ExprData) TestIsInDays(tm time.Time) error {
	weekday := ISOWeekday(tm)
	isInDay := func(day Day) error {
		if weekday != day.Weekday {
			return ErrNotAllowed
		}
		if day.Monthday == 0 {
			return nil
		}
		if tm.Day() != day.Monthday {
			return ErrNotAllowed
		}
		return nil
	}
	for _, day := range t.ContextualizeDays(tm) {
		if err := isInDay(day); err == nil {
			return nil
		} else if errors.Is(err, ErrNotAllowed) {
			// maybe next day is allowed
			continue
		} else {
			return err
		}
	}
	return errors.Wrap(ErrNotAllowed, "not in allowed days")
}

func (t *Expr) TestWithLast(tm time.Time, last time.Time) (time.Duration, error) {
	if err := t.makeDataset(); err != nil {
		return 0, err
	}

	// needActionInterval returns false if timestamp is fresher than now-interval
	// returns true otherwize.
	// Zero is a infinite interval.
	needActionInterval := func(delay time.Duration) bool {
		if delay == 0 {
			return false
		}
		if last.IsZero() {
			return true
		}
		limit := last.Add(delay)
		if tm == limit {
			return true
		}
		if tm.After(limit) {
			return true
		}
		return false
	}

	// isInTimerangeInterval validates the last task run is old enough to allow running again.
	isInTimerangeInterval := func(tr Timerange) error {
		if tr.Interval == 0 {
			return errors.Wrap(ErrNotAllowed, "interval set to 0")
		}
		if last.IsZero() {
			return nil
		}
		if !needActionInterval(tr.Interval) {
			return errors.Wrap(ErrNotAllowed, "last run too soon")
		}
		return nil
	}

	// timerangeRemainingDelay returns the delay from now to the end of the timerange
	timerangeRemainingDelay := func(tr Timerange) time.Duration {
		begin := tr.Begin
		end := tr.End
		seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
		if tr.Begin > tr.End {
			end += time.Hour * 24
		}
		if seconds < begin {
			seconds += time.Hour * 24
		}
		return end - seconds
	}

	//
	// isInTimeranges validates the timerange constraints of a schedule.
	// Iterates multiple allowed timeranges.
	//
	// Return a delay the caller should wait before executing the task,
	// with garanty the delay doesn't reach outside the valid timerange:
	//
	// * 0 => immediate execution
	// * n => the duration to wait
	//
	// Returns ErrNotAllowed if the validation fails the timerange
	// constraints.
	//
	isInTimeranges := func(d ExprData) (time.Duration, error) {
		if len(d.Timeranges) == 0 {
			return 0, errors.Wrap(ErrNotAllowed, "no timeranges")
		}
		ec := errchain.New()
		ec.Add(ErrNotAllowed)
		for _, tr := range d.Timeranges {
			if err := tr.TestIncludes(tm); errors.Is(err, ErrNotAllowed) {
				ec.Add(err)
				continue
			} else if err != nil {
				return 0, err
			} else if d.Exclude {
				return timerangeRemainingDelay(tr), nil
			}
			if err := isInTimerangeInterval(tr); errors.Is(err, ErrNotAllowed) {
				ec.Add(err)
				continue
			} else if err != nil {
				return 0, err
			} else {
				return tr.Delay(tm), nil
			}
		}
		return 0, ec
	}

	validate := func(d ExprData) (time.Duration, error) {
		if err := d.TestIsInMonths(tm); err != nil {
			return 0, err
		}
		if err := d.TestIsInWeeks(tm); err != nil {
			return 0, err
		}
		if err := d.TestIsInDays(tm); err != nil {
			return 0, err
		}
		return isInTimeranges(d)
	}

	if len(t.dataset) == 0 {
		return 0, errors.Wrap(ErrNotAllowed, "no schedule")
	}
	reasons := make([]string, 0)
	for _, d := range t.dataset {
		delay, err := validate(d)
		if errors.Is(err, ErrNotAllowed) {
			reasons = append(reasons, fmt.Sprint(err))
			continue
		}
		if err != nil {
			return delay, err
		}
		if d.Exclude {
			return delay, errors.Wrapf(ErrExcluded, "schedule element '%s', delay '%s'", d.Raw, delay)
		}
		return delay, nil
	}
	return 0, errors.Wrapf(ErrNotAllowed, "%+v", reasons)
}

func (t *Expr) Test(tm time.Time) (time.Duration, error) {
	return t.TestWithLast(tm, time.Time{})
}

func newExprDataset() ExprDataset {
	return make(ExprDataset, 0)
}

// Includes returns the filtered elements with .Exclude=false
func (t ExprDataset) Includes() ExprDataset {
	l := make(ExprDataset, 0)
	for _, data := range t {
		if !data.Exclude {
			l = append(l, data)
		}
	}
	return l
}

// Excludes returns the filtered elements with .Exclude=true
func (t ExprDataset) Excludes() ExprDataset {
	l := make(ExprDataset, 0)
	for _, data := range t {
		if data.Exclude {
			l = append(l, data)
		}
	}
	return l
}

func newExprData() *ExprData {
	data := &ExprData{
		Timeranges: make(Timeranges, 0),
		Days:       make([]Day, 0),
		Weeks:      make([]int, 0),
		Months:     make([]int, 0),
	}
	return data
}

func normalizeExpression(s string) []string {
	expressions := make([]string, 0)

	switch s {
	case "", "@0":
		return []string{}
	default:
		// may be in ["expr1", "expr2"] format
		// => align to the list of expression format
		if err := json.Unmarshal([]byte(s), &expressions); err == nil {
			return expressions
		}
	}
	return []string{s}
}

func parse(s string) (ExprDataset, error) {
	ds := newExprDataset()
	for _, expr := range normalizeExpression(s) {
		if data, err := parseExpr(expr); err != nil {
			return nil, err
		} else {
			ds = append(ds, data)
		}
	}
	return ds, nil
}

func parseExpr(s string) (ExprData, error) {
	var (
		exclude bool
		err     error
	)
	data := newExprData()
	data.Raw = s
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return *data, nil
	}
	if s[0] == []byte("!")[0] {
		exclude = true
		s = s[1:]
	}
	if len(s) == 0 {
		return *data, nil
	}
	elements := strings.Fields(s)
	switch len(elements) {
	case 1:
		if data.Timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		data.Days = AllDays
		data.Weeks = allWeeksThisYear()
		data.Months = AllMonths
	case 2:
		if data.Timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.Days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		data.Weeks = allWeeksThisYear()
		data.Months = AllMonths
	case 3:
		if data.Timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.Days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		if data.Weeks, err = parseWeeks(elements[2]); err != nil {
			return *data, err
		}
		data.Months = AllMonths
	case 4:
		if data.Timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.Days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		if data.Weeks, err = parseWeeks(elements[2]); err != nil {
			return *data, err
		}
		if data.Months, err = parseMonths(elements[3]); err != nil {
			return *data, err
		}
	default:
		return *data, errors.Wrapf(ErrInvalid, "number of elements must be between 1-4: %s", s)
	}
	data.Exclude = exclude
	return *data, nil
}

func parseTime(s string) (time.Duration, error) {
	elements := strings.Split(s, ":")
	t := time.Second * 0
	switch len(elements) {
	case 3:
		if i, err := strconv.Atoi(elements[0]); err == nil {
			t += time.Hour * time.Duration(i)
		} else {
			return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
		}
		if i, err := strconv.Atoi(elements[1]); err == nil {
			t += time.Minute * time.Duration(i)
		} else {
			return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
		}
		if i, err := strconv.Atoi(elements[2]); err == nil {
			t += time.Second * time.Duration(i)
		} else {
			return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
		}
		return t, nil
	case 2:
		if i, err := strconv.Atoi(elements[0]); err == nil {
			t += time.Hour * time.Duration(i)
		} else {
			return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
		}
		if i, err := strconv.Atoi(elements[1]); err == nil {
			t += time.Minute * time.Duration(i)
		} else {
			return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
		}
		return t, nil
	default:
		return 0, errors.Wrapf(ErrInvalid, "time: %s", s)
	}
}

func parseTimeranges(s string) (Timeranges, error) {
	minDuration := time.Second * 1
	parse := func(s string) (time.Duration, time.Duration, error) {
		var (
			begin, end       time.Duration
			beginStr, endStr string
			err              error
		)
		switch s {
		case "", "*":
			begin = time.Second * 0
			end = time.Hour*24 - 1
			return begin, end, nil
		}
		elements := strings.Split(s, "-")
		switch len(elements) {
		case 1:
			beginStr = s
			endStr = s
		case 2:
			beginStr = elements[0]
			endStr = elements[1]
		default:
			return 0, 0, errors.Wrapf(ErrInvalid, "too many '-' in timerange expression: %s", s)
		}
		if begin, err = parseTime(beginStr); err != nil {
			return 0, 0, err
		}
		if end, err = parseTime(endStr); err != nil {
			return 0, 0, err
		}
		if begin == end {
			end += minDuration
		}
		return begin, end, nil
	}
	l := make(Timeranges, 0)
	for _, spec := range strings.Split(s, ",") {
		tr := Timerange{
			End:      (time.Hour * 24) - 1,
			Interval: time.Hour * 24,
		}
		if len(spec) == 0 {
			l = append(l, tr)
			continue
		}
		if spec[0] == []byte("~")[0] {
			tr.Probabilistic = true
			spec = spec[1:]
		}
		if spec == "*" {
			l = append(l, tr)
			continue
		}
		elements := strings.Split(spec, "@")
		switch len(elements) {
		case 1:
			if begin, end, err := parse(spec); err != nil {
				return nil, err
			} else {
				tr.Begin = begin
				tr.End = end
				tr.Interval = defaultInterval(begin, end)
				if tr.Interval < (minDuration + 1) {
					tr.Probabilistic = false
				}
				l = append(l, tr)
				continue
			}
		case 2:
			var defInterval time.Duration
			if begin, end, err := parse(elements[0]); err != nil {
				return nil, err
			} else {
				tr.Begin = begin
				tr.End = end
				defInterval = defaultInterval(begin, end)
			}
			if _, err := strconv.Atoi(elements[1]); err == nil {
				// no unit specified (ex: @10)
				// assume minutes for backward compat
				elements[1] += "m"
			}
			if interval, err := converters.Duration.Convert(elements[1]); err != nil {
				return nil, errors.Wrapf(ErrInvalid, "%s", err)
			} else {
				tr.Interval = *interval.(*time.Duration)
				if tr.Interval == 0 {
					// discard '...@0' timerange
					continue
				}
				if defInterval < (minDuration+1) || tr.Interval < defInterval {
					tr.Probabilistic = false
				}
				l = append(l, tr)
				continue
			}
		default:
			return nil, errors.Wrapf(ErrInvalid, "only one @<interval> allowed: %s", spec)
		}
	}
	return l, nil
}

func defaultInterval(begin, end time.Duration) time.Duration {
	if begin < end {
		return end - begin + 1
	}
	return (time.Hour*24 - begin) + end + 1
}

func parseDay(s string) ([]Day, error) {
	dayOfWeekStr := s
	dayOfMonth := 0
	elements := strings.Split(s, ":")
	switch len(elements) {
	case 1:
		// pass
	case 2:
		dayOfWeekStr = elements[0]
		dayOfMonthStr := elements[1]
		if len(dayOfMonthStr) == 0 {
			return nil, errors.Wrapf(ErrInvalid, "day_of_month specifier is empty: %s", s)
		}
		switch dayOfMonthStr {
		case "first", "1st":
			dayOfMonth = 1
		case "second", "2nd":
			dayOfMonth = 2
		case "third", "3rd":
			dayOfMonth = 3
		case "fourth", "4th":
			dayOfMonth = 4
		case "fifth", "5th":
			dayOfMonth = 5
		case "last":
			dayOfMonth = -1
		default:
			if i, err := strconv.Atoi(dayOfMonthStr); err == nil {
				dayOfMonth = i
				if dayOfMonth == 0 {
					return nil, errors.Wrapf(ErrInvalid, "day_of_month expression not supported: %s", s)
				}
			} else {
				return nil, errors.Wrapf(ErrInvalid, "day_of_month expression not supported: %s", s)
			}
		}
	default:
		return nil, errors.Wrapf(ErrInvalid, "only one ':' allowed in day spec: %s", s)
	}
	days, err := parseWeekdays(dayOfWeekStr)
	if err != nil {
		return nil, err
	}
	l := make([]Day, len(days))
	for i, d := range days {
		l[i].Monthday = dayOfMonth
		l[i].Weekday = d
	}
	return l, nil
}

func parseDays(s string) ([]Day, error) {
	l := make([]Day, 0)
	for _, sub := range strings.Split(s, ",") {
		if more, err := parseDay(sub); err == nil {
			l = append(l, more...)
		} else {
			return nil, err
		}
	}
	return l, nil
}

func resolveCalendarName(s string) (int, error) {
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	s = strings.ToLower(s)
	if i, ok := CalendarNames[s]; !ok {
		return -1, errors.Wrapf(ErrInvalid, "unknown calendar name: %s", s)
	} else {
		return i, nil
	}
}

//
// parseCalendarExpression is the top level schedule definition parser.
// It splits the definition into sub-schedules, and parses each one.
//
func parseCalendarExpression(spec string, all []int) ([]int, error) {
	merge := func(a, b map[int]interface{}) map[int]interface{} {
		for k, v := range b {
			a[k] = v
		}
		return a
	}
	list := func(a map[int]interface{}) []int {
		l := make([]int, 0)
		for k, _ := range a {
			l = append(l, k)
		}
		sort.Ints(l)
		return l
	}

	m := make(map[int]interface{})
	switch spec {
	case "*", "":
		return all, nil
	default:
		subSpecs := strings.Split(spec, ",")
		for _, subSpec := range subSpecs {
			if other, err := parseOneCalendarExpression(subSpec, all); err != nil {
				return nil, err
			} else {
				m = merge(m, other)
			}
		}
		return list(m), nil
	}
}
func parseOneCalendarExpression(spec string, all []int) (map[int]interface{}, error) {
	m := make(map[int]interface{})
	if len(spec) < 1 {
		return m, nil
	}
	nDash := strings.Count(spec[1:], "-")
	switch nDash {
	case 0:
		if i, err := resolveCalendarName(spec); err != nil {
			return m, err
		} else {
			m[i] = nil
			return m, nil
		}
	case 1:
		l := strings.Split(spec, "-")
		var (
			begin, end int
			err        error
		)
		if begin, err = resolveCalendarName(l[0]); err != nil {
			return m, err
		}
		if end, err = resolveCalendarName(l[1]); err != nil {
			return m, err
		}
		if begin > end {
			// beware: no zero element:
			//  days are 1 to 7
			//  weeks are 1 to 53
			//  months are 1 to 12
			for i := begin; i <= len(all); i += 1 {
				m[i] = nil
			}
			for i := 1; i <= end; i += 1 {
				m[i] = nil
			}
		} else {
			for i := begin; i <= end; i += 1 {
				m[i] = nil
			}
		}
		return m, nil
	default:
		return m, errors.Wrapf(ErrInvalid, "only one '-' is allowed in range: %s", spec)
	}
	return m, errors.Wrapf(ErrInvalid, "unexpected syntax: %s", spec)
}
func parseMonths(spec string) ([]int, error) {
	elements := strings.Split(spec, "%")
	switch len(elements) {
	case 1:
		return parseCalendarExpression(spec, AllMonths)
	case 2:
		months, err := parseCalendarExpression(elements[0], AllMonths)
		if err != nil {
			return nil, err
		}
		return filterWithModulo(months, elements[1])
	default:
		return nil, errors.Wrapf(ErrInvalid, "too many '%%': %s", spec)
	}
}
func parseWeekdays(spec string) ([]int, error) {
	return parseCalendarExpression(spec, AllWeekdays)
}
func parseWeeks(spec string) ([]int, error) {
	return parseCalendarExpression(spec, allWeeksThisYear())
}

func lastWeek(tm time.Time) int {
	sylvester := time.Date(tm.Year(), time.December, 31, 12, 00, 00, 00, tm.Location())
	_, i := sylvester.ISOWeek()
	return i
}

func allWeeksThisYear() []int {
	return allWeeks(time.Now())
}

func allWeeks(tm time.Time) []int {
	i := lastWeek(tm)
	return seq.Ints(1, i)
}

func parseModulo(s string) (int, int, error) {
	var modulo, shift int
	var err error
	switch strings.Count(s, "+") {
	case 1:
		l := strings.SplitN(s, "+", 2)
		if shift, err = strconv.Atoi(l[1]); err != nil {
			return 0, 0, errors.Wrapf(ErrInvalid, "modulo shift must be an int: %s", s)
		}
		s = l[0]
	case 0:
		shift = 0
	default:
		return 0, 0, errors.Wrapf(ErrInvalid, "only one '+' is allowed in modulo: %s", s)
	}
	if modulo, err = strconv.Atoi(s); err != nil {
		return 0, 0, errors.Wrapf(ErrInvalid, "modulo must be an int: %s", s)
	}
	return modulo, shift, nil
}

func filterWithModulo(l []int, s string) ([]int, error) {
	filtered := make([]int, 0)
	modulo, shift, err := parseModulo(s)
	if err != nil {
		return nil, err
	}
	for _, v := range l {
		if ((v + shift) % modulo) == 0 {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
}

//
// (*Expr).Next() implementation
//

func NextWithLast(tm time.Time) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*nextOptionsT)
		t.Last = tm
		return nil
	})
}

func NextWithTime(tm time.Time) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*nextOptionsT)
		t.Time = tm
		return nil
	})
}

type nextOptionsT struct {
	Time time.Time
	Last time.Time
}

func newNextOptions(opts ...funcopt.O) nextOptionsT {
	options := nextOptionsT{
		Time: time.Now(),
	}
	_ = funcopt.Apply(&options, opts...)
	return options
}

// Next returns the next time the expression is valid, and the duration
// after that time the schedule is denied.
func (t *Expr) Next(opts ...funcopt.O) (time.Time, time.Duration, error) {
	var (
		next     time.Time
		interval time.Duration
	)
	options := newNextOptions(opts...)
	if err := t.makeDataset(); err != nil {
		return next, interval, err
	}
	excludes := t.dataset.Excludes()
	for _, data := range t.dataset.Includes() {
		_next, _interval := getNext(data, options, excludes)
		if next.IsZero() || next.After(_next) {
			next = _next
			interval = _interval
		}
	}
	return next, interval, nil
}

func getNext(data ExprData, options nextOptionsT, excludes ExprDataset) (time.Time, time.Duration) {
	var (
		next     time.Time
		interval time.Duration
	)

	isValidDay := func(tm time.Time, days []Day) bool {
		weekday := ISOWeekday(tm)
		monthday := int(tm.Day())
		for _, d := range days {
			if d.Weekday != weekday {
				continue
			}
			if d.Monthday == 0 {
				return true
			}
			if d.Monthday == monthday {
				return true
			}
		}
		return false
	}

	validate := func(tm time.Time) (time.Duration, error) {
		expr := New(data.Raw)
		expr.AppendExprDataset(excludes)
		return expr.TestWithLast(tm, options.Last)
	}

	nextDay := func(tm time.Time) (time.Time, time.Duration, error) {
		tm = tm.AddDate(0, 0, 1) // next day
		tm = time.Date(          // at 00:00:00
			tm.Year(), tm.Month(), tm.Day(),
			0, 0, 0, 0,
			tm.Location(),
		)
		return tm, 0, ErrNextDay
	}

	daily := func(tm time.Time, days []Day) (time.Time, time.Duration, error) {
		var err error
		if !data.IsInWeeks(tm) {
			return nextDay(tm)
		}
		if !isValidDay(tm, days) {
			return nextDay(tm)
		}
		if tm, interval, err = data.GetTimerange(tm, options.Last); errors.Is(err, ErrNotAllowed) {
			return nextDay(tm)
		}
		if interval, err = validate(tm); errors.Is(err, ErrNotAllowed) {
			// pass
		} else if errors.Is(err, ErrExcluded) {
			if interval > 0 {
				tmi := tm.Add(interval)
				if tm.YearDay() == tmi.YearDay() {
					return tmi, interval, ErrDrift
				}
				return nextDay(tm)
			}
		}
		return tm, interval, nil
	}

	tm := options.Time
	year1 := int(tm.Year())
	month1 := int(tm.Month())
	for year := year1; year <= year1+1; year += 1 {
		for _, month := range data.Months {
			var firstDay int
			if year == year1 {
				if month < month1 {
					// skip the head months until now
					continue
				} else if month > month1 {
					// skipped beyond initial time (due to %<expr> for ex)
					tm = time.Date(
						year, time.Month(month), 1,
						0, 0, 0, 0,
						tm.Location(),
					)
				}
			} else {
				// skipped beyond initial time (due to month range for ex)
				tm = time.Date(
					year, time.Month(month), 1,
					0, 0, 0, 0,
					tm.Location(),
				)
			}

			if (year == year1) && (month == month1) {
				firstDay = tm.Day()
			} else {
				firstDay = 1
			}
			for _, monthday := range seq.Ints(firstDay, monthDays(tm)) {
				tm = time.Date(
					year, time.Month(month), monthday,
					tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(),
					tm.Location(),
				)
				days := data.ContextualizeDays(tm)
				for {
					tmi, interval, err := daily(tm, days)
					if errors.Is(err, ErrDrift) {
						tm = tmi
						continue
					}
					if errors.Is(err, ErrNextDay) {
						tm = tmi
						break
					}
					return tmi, interval
				}
			}

		}
	}
	return next, interval
}
