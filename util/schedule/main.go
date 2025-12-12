package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	errchain "github.com/g8rswimmer/error-chain"

	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/seq"
)

var (
	getNow = time.Now

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
	AllDays     = []day{
		{weekday: 1},
		{weekday: 2},
		{weekday: 3},
		{weekday: 4},
		{weekday: 5},
		{weekday: 6},
		{weekday: 7},
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
	timerange struct {
		probabilistic bool
		interval      time.Duration
		begin         time.Duration // since 00:00:00
		end           time.Duration // since 00:00:00
	}
	timeranges []timerange
	day        struct {
		weekday  int
		monthday int
	}

	// Schedule is a single parsed scheduling expression
	Schedule struct {
		timeranges timeranges
		days       []day
		weeks      []int
		months     []int
		raw        string
		exclude    bool
	}

	// Schedules is a list of Schedule, applying a union logic to allowed ranges
	Schedules []Schedule

	Expr struct {
		raw     string
		dataset Schedules
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

func (t *Expr) AppendExprDataset(ds Schedules) {
	for _, data := range ds {
		t.raw = strings.Join([]string{t.raw, data.raw}, " ")
		t.dataset = append(t.dataset, data)
	}
}

func (t Expr) String() string {
	return t.raw
}

func (t *Expr) Dataset() Schedules {
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

// contextualizeDays returns a copy of Days with the "nth-from-tail" monthdays
// evaluated using the actual number of days in the <tm> month
func (t Schedule) contextualizeDays(tm time.Time) []day {
	maxValue := monthDays(tm)
	days := make([]day, len(t.days))
	for i, d := range t.days {
		days[i] = d
		if d.monthday == 0 {
			continue
		}
		if d.monthday > 0 {
			continue
		}
		if -d.monthday > maxValue {
			continue
		}
		days[i].monthday = maxValue + d.monthday + 1
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
func (tr timerange) After(tm time.Time) bool {
	begin := tr.begin
	seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
	return begin >= seconds
}

// TestIncludes returns ErrNotAllowed if <tm> is not in the Timerange
func (tr timerange) TestIncludes(tm time.Time) error {
	begin := tr.begin
	end := tr.end
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
	return fmt.Errorf("%w: not in timerange %s-%s", ErrNotAllowed, tr.begin, tr.end)
}

// Includes returns true if <tm> is in the Timerange
func (tr timerange) Includes(tm time.Time) bool {
	err := tr.TestIncludes(tm)
	return err == nil
}

// Delay returns a delay in seconds, compatible with the timerange.
//
// The daemon scheduler thread will honor this delay,
// executing the task only when expired.
//
// This algo is meant to level collector's load which peaks
// when tasks trigger at the same second on every nodes.
func (tr timerange) Delay(tm time.Time) time.Duration {
	if !tr.probabilistic {
		return 0
	}
	begin := tr.begin
	end := tr.end
	seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
	if tr.begin > tr.end {
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

	if tr.interval < length {
		// don't delay if interval < period length, because the user
		// expects the action to run multiple times in the period. And
		// '@<n>' interval-only schedule are already different across
		// nodes due to daemons not starting at the same moment.
		return 0
	}

	return time.Duration(float64(remaining) * rand.Float64())
}

func (t timeranges) Including(tm time.Time) timeranges {
	trs := make(timeranges, 0)
	for _, tr := range t {
		if !tr.Includes(tm) {
			continue
		}
		trs = append(trs, tr)
	}
	return trs
}

func (t timeranges) After(tm time.Time) timeranges {
	trs := make(timeranges, 0)
	for _, tr := range t {
		if !tr.After(tm) {
			continue
		}
		trs = append(trs, tr)
	}
	return trs
}

func (t timeranges) SortedByIntervalAndBegin() timeranges {
	trs := make(timeranges, len(t))
	for i, tr := range t {
		trs[i] = tr
	}
	sort.Slice(trs, func(i, j int) bool {
		switch {
		case trs[i].interval < trs[j].interval:
			return true
		case trs[i].interval == trs[j].interval:
			return trs[i].interval < trs[j].interval
		default:
			return false
		}
	})
	return trs
}

func (t Schedule) GetTimerange(tm, last time.Time) (time.Time, time.Duration, error) {
	// if the candidate date is inside timeranges, return (candidate, smallest interval)
	trs := t.timeranges.Including(tm).SortedByIntervalAndBegin()
	for _, tr := range trs {
		if isTooSoon(tm, last, tr.interval) {
			tmi := last.Add(tr.interval)
			if tr.Includes(tmi) {
				return tmi, tr.interval, nil
			}
			return tm, 0, ErrNotAllowed
		}
		if tr.probabilistic {
			delay := tr.Delay(tm)
			tmi := tm.Add(delay)
			return tmi, tr.interval - delay, nil
		}
		return tm, tr.interval, nil
	}

	// the candidate date is outside timeranges, return the closest range's (begin, interval)
	trs = t.timeranges.After(tm).SortedByIntervalAndBegin()
	for _, tr := range trs {
		tm := time.Date(
			tm.Year(), tm.Month(), tm.Day(),
			0, 0, 0, 0,
			tm.Location(),
		).Add(tr.begin)
		if isTooSoon(tm, last, tr.interval) {
			tmi := last.Add(tr.interval)
			if tr.Includes(tmi) {
				return tmi, tr.interval, nil
			}
			continue
		}
		if tr.probabilistic {
			delay := tr.Delay(tm)
			tmi := tm.Add(delay)
			return tmi, tr.interval - delay, nil
		}
		return tm, tr.interval, nil
	}

	return tm, 0, ErrNotAllowed
}

func (t Schedule) IsInWeeks(tm time.Time) bool {
	err := t.TestIsInWeeks(tm)
	return err == nil
}

func (t Schedule) TestIsInWeeks(tm time.Time) error {
	_, ref := tm.ISOWeek()
	for _, week := range t.weeks {
		if week == ref {
			return nil
		}
	}
	return fmt.Errorf("%w: not in allowed weeks", ErrNotAllowed)
}

func (t Schedule) IsInMonths(tm time.Time) bool {
	err := t.TestIsInMonths(tm)
	return err == nil
}

func (t Schedule) TestIsInMonths(tm time.Time) error {
	ref := int(tm.Month())
	for _, month := range t.months {
		if month == ref {
			return nil
		}
	}
	return fmt.Errorf("%w: not in allowed months", ErrNotAllowed)
}

func (t Schedule) IsInDays(tm time.Time) bool {
	err := t.TestIsInDays(tm)
	return err == nil
}

func (t Schedule) TestIsInDays(tm time.Time) error {
	weekday := ISOWeekday(tm)
	isInDay := func(d day) error {
		if weekday != d.weekday {
			return ErrNotAllowed
		}
		if d.monthday == 0 {
			return nil
		}
		if tm.Day() != d.monthday {
			return ErrNotAllowed
		}
		return nil
	}
	for _, d := range t.contextualizeDays(tm) {
		if err := isInDay(d); err == nil {
			return nil
		} else if errors.Is(err, ErrNotAllowed) {
			// maybe next day is allowed
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("%w: not in allowed days", ErrNotAllowed)
}

func (t *Expr) TestWithLast(tm time.Time, last time.Time) (time.Duration, error) {
	if err := t.makeDataset(); err != nil {
		return 0, err
	}

	// needActionInterval returns false if timestamp is fresher than now-interval
	// returns true otherwise.
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
	isInTimerangeInterval := func(tr timerange) error {
		if tr.interval == 0 {
			return fmt.Errorf("%w: the interval is set to 0", ErrNotAllowed)
		}
		if last.IsZero() {
			return nil
		}
		if !needActionInterval(tr.interval) {
			return fmt.Errorf("%w: the last run is too soon", ErrNotAllowed)
		}
		return nil
	}

	// timerangeRemainingDelay returns the delay from now to the end of the timerange
	timerangeRemainingDelay := func(tr timerange) time.Duration {
		begin := tr.begin
		end := tr.end
		seconds := tm.Sub(time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, tm.Location()))
		if tr.begin > tr.end {
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
	// with guarantee the delay doesn't reach outside the valid timerange:
	//
	// * 0 => immediate execution
	// * n => the duration to wait
	//
	// Returns ErrNotAllowed if the validation fails the timerange
	// constraints.
	//
	isInTimeranges := func(d Schedule) (time.Duration, error) {
		if len(d.timeranges) == 0 {
			return 0, fmt.Errorf("%w: no timeranges", ErrNotAllowed)
		}
		ec := errchain.New()
		ec.Add(ErrNotAllowed)
		for _, tr := range d.timeranges {
			if err := tr.TestIncludes(tm); errors.Is(err, ErrNotAllowed) {
				ec.Add(err)
				continue
			} else if err != nil {
				return 0, err
			} else if d.exclude {
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

	validate := func(d Schedule) (time.Duration, error) {
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
		return 0, fmt.Errorf("%w: no schedule", ErrNotAllowed)
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
		if d.exclude {
			return delay, fmt.Errorf("%w: schedule element '%s', delay '%s'", ErrExcluded, d.raw, delay)
		}
		return delay, nil
	}
	return 0, fmt.Errorf("%w: %+v", ErrNotAllowed, reasons)
}

func (t *Expr) Test(tm time.Time) (time.Duration, error) {
	return t.TestWithLast(tm, time.Time{})
}

func newExprDataset() Schedules {
	return make(Schedules, 0)
}

// Includes returns the filtered elements with .Exclude=false
func (t Schedules) Includes() Schedules {
	l := make(Schedules, 0)
	for _, data := range t {
		if !data.exclude {
			l = append(l, data)
		}
	}
	return l
}

// Excludes returns the filtered elements with .Exclude=true
func (t Schedules) Excludes() Schedules {
	l := make(Schedules, 0)
	for _, data := range t {
		if data.exclude {
			l = append(l, data)
		}
	}
	return l
}

func newExprData() *Schedule {
	data := &Schedule{
		timeranges: make(timeranges, 0),
		days:       make([]day, 0),
		weeks:      make([]int, 0),
		months:     make([]int, 0),
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

func parse(s string) (Schedules, error) {
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

func parseExpr(s string) (Schedule, error) {
	var (
		exclude bool
		err     error
	)
	data := newExprData()
	data.raw = s
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
		if data.timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		data.days = AllDays
		data.weeks = allWeeksThisYear()
		data.months = AllMonths
	case 2:
		if data.timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		data.weeks = allWeeksThisYear()
		data.months = AllMonths
	case 3:
		if data.timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		if data.weeks, err = parseWeeks(elements[2]); err != nil {
			return *data, err
		}
		data.months = AllMonths
	case 4:
		if data.timeranges, err = parseTimeranges(elements[0]); err != nil {
			return *data, err
		}
		if data.days, err = parseDays(elements[1]); err != nil {
			return *data, err
		}
		if data.weeks, err = parseWeeks(elements[2]); err != nil {
			return *data, err
		}
		if data.months, err = parseMonths(elements[3]); err != nil {
			return *data, err
		}
	default:
		return *data, fmt.Errorf("%w: the number of elements must be between 1-4: %s", ErrInvalid, s)
	}
	data.exclude = exclude
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
			return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
		}
		if i, err := strconv.Atoi(elements[1]); err == nil {
			t += time.Minute * time.Duration(i)
		} else {
			return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
		}
		if i, err := strconv.Atoi(elements[2]); err == nil {
			t += time.Second * time.Duration(i)
		} else {
			return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
		}
		return t, nil
	case 2:
		if i, err := strconv.Atoi(elements[0]); err == nil {
			t += time.Hour * time.Duration(i)
		} else {
			return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
		}
		if i, err := strconv.Atoi(elements[1]); err == nil {
			t += time.Minute * time.Duration(i)
		} else {
			return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
		}
		return t, nil
	default:
		return 0, fmt.Errorf("%w: time: %s", ErrInvalid, s)
	}
}

func parseTimeranges(s string) (timeranges, error) {
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
			return 0, 0, fmt.Errorf("%w: too many '-' in timerange expression: %s", ErrInvalid, s)
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
	l := make(timeranges, 0)
	for _, spec := range strings.Split(s, ",") {
		tr := timerange{
			end:      (time.Hour * 24) - 1,
			interval: time.Hour * 24,
		}
		if len(spec) == 0 {
			l = append(l, tr)
			continue
		}
		if spec[0] == []byte("~")[0] {
			tr.probabilistic = true
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
				tr.begin = begin
				tr.end = end
				tr.interval = defaultInterval(begin, end)
				if tr.interval < (minDuration + 1) {
					tr.probabilistic = false
				}
				l = append(l, tr)
				continue
			}
		case 2:
			var defInterval time.Duration
			if begin, end, err := parse(elements[0]); err != nil {
				return nil, err
			} else {
				tr.begin = begin
				tr.end = end
				defInterval = defaultInterval(begin, end)
			}
			if _, err := strconv.Atoi(elements[1]); err == nil {
				// no unit specified (ex: @10)
				// assume minutes for backward compat
				elements[1] += "m"
			}
			if interval, err := converters.Lookup("duration").Convert(elements[1]); err != nil {
				return nil, fmt.Errorf("%w: %s", ErrInvalid, err)
			} else {
				tr.interval = *interval.(*time.Duration)
				if tr.interval == 0 {
					// discard '...@0' timerange
					continue
				}
				if defInterval < (minDuration+1) || tr.interval < defInterval {
					tr.probabilistic = false
				}
				l = append(l, tr)
				continue
			}
		default:
			return nil, fmt.Errorf("%w: only one @<interval> allowed: %s", ErrInvalid, spec)
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

func parseDay(s string) ([]day, error) {
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
			return nil, fmt.Errorf("%w: the day_of_month specifier is empty: %s", ErrInvalid, s)
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
					return nil, fmt.Errorf("%w: the day_of_month expression not supported: %s", ErrInvalid, s)
				}
			} else {
				return nil, fmt.Errorf("%w: the day_of_month expression not supported: %s", ErrInvalid, s)
			}
		}
	default:
		return nil, fmt.Errorf("%w: only one ':' allowed in day spec: %s", ErrInvalid, s)
	}
	days, err := parseWeekdays(dayOfWeekStr)
	if err != nil {
		return nil, err
	}
	l := make([]day, len(days))
	for i, d := range days {
		l[i].monthday = dayOfMonth
		l[i].weekday = d
	}
	return l, nil
}

func parseDays(s string) ([]day, error) {
	l := make([]day, 0)
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
		return -1, fmt.Errorf("%w: unknown calendar name: %s", ErrInvalid, s)
	} else {
		return i, nil
	}
}

// parseCalendarExpression is the top level schedule definition parser.
// It splits the definition into sub-schedules, and parses each one.
func parseCalendarExpression(spec string, all []int) ([]int, error) {
	merge := func(a, b map[int]interface{}) map[int]interface{} {
		for k, v := range b {
			a[k] = v
		}
		return a
	}
	list := func(a map[int]interface{}) []int {
		l := make([]int, 0)
		for k := range a {
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
			for i := begin; i <= len(all); i++ {
				m[i] = nil
			}
			for i := 1; i <= end; i++ {
				m[i] = nil
			}
		} else {
			for i := begin; i <= end; i++ {
				m[i] = nil
			}
		}
		return m, nil
	default:
		return m, fmt.Errorf("%w: only one '-' is allowed in range: %s", ErrInvalid, spec)
	}
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
		return nil, fmt.Errorf("%w: too many '%%': %s", ErrInvalid, spec)
	}
}
func parseWeekdays(spec string) ([]int, error) {
	return parseCalendarExpression(spec, AllWeekdays)
}
func parseWeeks(spec string) ([]int, error) {
	return parseCalendarExpression(spec, allWeeksThisYear())
}

func lastWeek(tm time.Time) int {
	d := time.Date(tm.Year(), time.December, 28, 12, 00, 00, 00, tm.Location())
	_, i := d.ISOWeek()
	return i
}

func allWeeksThisYear() []int {
	return allWeeks(getNow())
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
			return 0, 0, fmt.Errorf("%w: modulo shift must be an int: %s", ErrInvalid, s)
		}
		s = l[0]
	case 0:
		shift = 0
	default:
		return 0, 0, fmt.Errorf("%w: only one '+' is allowed in modulo: %s", ErrInvalid, s)
	}
	if modulo, err = strconv.Atoi(s); err != nil {
		return 0, 0, fmt.Errorf("%w: modulo must be an int: %s", ErrInvalid, s)
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
		Time: getNow(),
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

func getNext(data Schedule, options nextOptionsT, excludes Schedules) (time.Time, time.Duration) {
	var (
		next     time.Time
		interval time.Duration
	)

	isValidDay := func(tm time.Time, days []day) bool {
		weekday := ISOWeekday(tm)
		monthday := tm.Day()
		for _, d := range days {
			if d.weekday != weekday {
				continue
			}
			if d.monthday == 0 {
				return true
			}
			if d.monthday == monthday {
				return true
			}
		}
		return false
	}

	validate := func(tm time.Time) (time.Duration, error) {
		expr := New(data.raw)
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

	daily := func(tm time.Time, days []day) (time.Time, time.Duration, error) {
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
	year1 := tm.Year()
	month1 := int(tm.Month())
	for year := year1; year <= year1+1; year++ {
		for _, month := range data.months {
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
				days := data.contextualizeDays(tm)
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
