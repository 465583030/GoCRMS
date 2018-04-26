package common

import (
	"time"
	"fmt"
	"strconv"
	"errors"
	"strings"
)

type LogItem struct {
	Time time.Time
	Msg string
	relative time.Duration  // the relative time to the previous log item
}

func (this *LogItem) eq(that *LogItem, threshold time.Duration) error {
	if this == that {
		return nil
	}
	if this.Msg != that.Msg {
		return errors.New(fmt.Sprintf("msg not eq: %s != %s", this.Msg, that.Msg))
	}
	diff := this.relative - that.relative
	if diff < 0 {
		diff = -diff
	}
	if diff > threshold {
		return errors.New(fmt.Sprintf(
			"relative diff > threshold: |%v - %v| > %v", this.relative, that.relative, threshold))
	}
	return nil
}
func (this *LogItem) swapMsg(that *LogItem) {
	this.Msg, that.Msg = that.Msg, this.Msg
}

func parseLogItem(line string) (*LogItem, error) {
	// date format: yyyy/MM/dd hh:mm:ss.SSSSSS
	const layout = "2018/04/24 13:44:52.874585"
	const nLayout = len(layout)
	if len(line) < nLayout {
		return nil, errors.New("line is too short to hold the time message")
	}
	// time.Parse() will return error: month out of range, so write manually
	// t, err := time.Parse(layout, line[:nLayout])
	t, err := parseTime(line[:nLayout])
	if err != nil {
		return nil, err
	}
	msg := strings.TrimLeft(line[nLayout:], " \t")
	return &LogItem{
		Time: t,
		Msg: msg,
	}, nil
}

type Log []*LogItem

func (this Log) Eq(that Log, threshold time.Duration) error {
	n := len(this)
	if n != len(that) {
		return errors.New(fmt.Sprintf("len not eq, %d != %d", n, len(that)))
	}
L:	for i := 0; i < n; i++ {
		if err := this[i].eq(that[i], threshold); err != nil {
			// when msg not eq, maybe the order is different
			if this[i].Msg != that[i].Msg {
				// compare with next if near
				for j := i + 1; j < n && that[j].Time.Sub(that[i].Time) < threshold; j++ {
					if this[i].eq(that[j], threshold) == nil {
						that[i].swapMsg(that[j])
						continue L
					}
				}
			}
			return errors.New(fmt.Sprintf("Log item %d, reason: %v", i + 1, err.Error()))
		}
	}
	return nil
}

func ParseLog(log string) Log {
	log = strings.Trim(log, " \r\n\t")
	lines := strings.Split(log, "\n")
	items := make([]*LogItem, 0, len(lines))
	for _, line := range lines {
		line = strings.Trim(line, " \r\t")
		if item, err := parseLogItem(line); err != nil {
			if len(items) > 0 {
				last := items[len(items)-1]
				last.Msg += "\n" + line
			}
		} else {
			items = append(items, item)
		}
	}
	// calculate relative time to the previous log item
	if len(items) > 1 {
		for i := 1; i < len(items); i++ {
			items[i].relative = items[i].Time.Sub(items[i-1].Time)
		}
	}
	return items
}

func EqLog(actual string, expected string, threshold time.Duration) error {
	aLog := ParseLog(actual)
	eLog := ParseLog(expected)
	return aLog.Eq(eLog, threshold)
}

func EqLogWithGolden(goldenFile, actual string, threshold time.Duration) error {
	return EqByFuncWithGolden(goldenFile, actual, func(actualValue, expected string) (bool, error) {
		err := EqLog(actualValue, expected, threshold)
		return err == nil, err
	})
}

// parse time format: yyyy/MM/dd hh:mm:ss.SSSSSS
// the input string's size should be the same with the above format, not check here.
// as time.Parse() will return error: month out of range, so write manually
func parseTime(s string) (t time.Time, err error) {
	wides := [...]int{4,2,2,2,2,2,6}
	var dts [len(wides)]int
	start := 0
	for k, wide := range wides {
		end := start + wide
		dts[k], err = strconv.Atoi(string(s[start:end]))
		if err != nil {
			return
		}
		start = end + 1
	}
	t = time.Date(dts[0], time.Month(dts[1]), dts[2], dts[3], dts[4], dts[5],
		int(time.Duration(dts[6]) * time.Microsecond), time.UTC)
	return
}
