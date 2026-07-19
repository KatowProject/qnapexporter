package prometheus

import (
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type smartDisk struct {
	Name     string
	Device   string
	Driver   string
	Model    string
	Serial   string
	WWN      string
	Firmware string
	IsSSD    bool
	IsNVMe   bool
	IsSAS    bool
	Health   float64
	Status   int // 0=OK, 1=WARNING, 2=CRITICAL

	// Common
	Temperature float64
	PowerOnHrs  float64
	PowerCycles float64

	// NVMe specific
	CriticalWarning      float64
	AvailableSpare       float64
	AvailableSpareThresh float64
	PercentageUsed       float64
	DataUnitsRead        float64
	DataUnitsWritten     float64
	HostReadCommands     float64
	HostWriteCommands    float64
	ControllerBusyTime   float64
	UnsafeShutdowns      float64
	MediaErrors          float64
	ErrorLogEntries      float64
	WarningTempTime      float64
	CriticalTempTime     float64
	WarningTempThresh    float64
	CriticalTempThresh   float64
	TempSensor1          float64
	TempSensor2          float64

	// SATA SSD specific
	WearLevelingValue   float64
	AvailableResvdValue float64
	WearLevelingRaw     float64
	TotalLBAsWritten    float64
	TotalLBAsRead       float64
	EraseFailCountChip  float64
	EraseFailCountTotal float64
	ProgramFailCntTotal float64
	UsedRsvdBlkCntChip  float64
	PowerOffRetract     float64

	// Common SATA (SSD + HDD)
	RawReadErrorRate     float64
	ReallocatedSectors   float64
	PendingSectors       float64
	OfflineUncorrectable float64
	UDMACRCErrors        float64
	RuntimeBadBlock      float64
	ReportedUncorrect    float64
	EndToEndErrors       float64

	// HDD only
	SpinRetryCount float64

	// SAS specific
	HealthStatus            string
	DriveTripTemperature    float64
	ReassignedBlocks        float64
	GrownDefectList         float64
	NonMediumErrors         float64
	ReadUncorrectedErrors   float64
	WriteUncorrectedErrors  float64
	VerifyUncorrectedErrors float64
	InvalidDwordCount       float64
	RunningDisparityErrors  float64
	LossDwordSyncCount      float64
	PhyResetProblemCount    float64
}

type smartDevice struct {
	Path   string
	Driver string
}

func initSmartDisk() smartDisk {
	return smartDisk{
		Temperature: -1, PowerOnHrs: -1, PowerCycles: -1,
		CriticalWarning: -1, AvailableSpare: -1, AvailableSpareThresh: -1, PercentageUsed: -1,
		DataUnitsRead: -1, DataUnitsWritten: -1, HostReadCommands: -1,
		HostWriteCommands: -1, ControllerBusyTime: -1, UnsafeShutdowns: -1,
		MediaErrors: -1, ErrorLogEntries: -1,
		WarningTempTime: -1, CriticalTempTime: -1, WarningTempThresh: -1, CriticalTempThresh: -1,
		TempSensor1: -1, TempSensor2: -1,
		WearLevelingValue: -1, AvailableResvdValue: -1, WearLevelingRaw: -1,
		TotalLBAsWritten: -1, TotalLBAsRead: -1,
		EraseFailCountChip: -1, EraseFailCountTotal: -1,
		ProgramFailCntTotal: -1, UsedRsvdBlkCntChip: -1, PowerOffRetract: -1,
		RawReadErrorRate: -1, ReallocatedSectors: -1, PendingSectors: -1,
		OfflineUncorrectable: -1, UDMACRCErrors: -1, RuntimeBadBlock: -1,
		ReportedUncorrect: -1, EndToEndErrors: -1, SpinRetryCount: -1,
		// SAS
		DriveTripTemperature: -1,
		ReassignedBlocks:     -1, GrownDefectList: -1,
		NonMediumErrors:       -1,
		ReadUncorrectedErrors: -1, WriteUncorrectedErrors: -1, VerifyUncorrectedErrors: -1,
		InvalidDwordCount: -1, RunningDisparityErrors: -1,
		LossDwordSyncCount: -1, PhyResetProblemCount: -1,
	}
}

func isSolidState(info string) bool {
	lower := strings.ToLower(info)
	return strings.Contains(lower, "solid state") ||
		strings.Contains(lower, "ssd") ||
		strings.Contains(info, "Flash")
}

func extractSmart(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return -1
}

func extractSmartComma(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		clean := strings.ReplaceAll(m[1], ",", "")
		v, _ := strconv.ParseFloat(clean, 64)
		return v
	}
	return -1
}

func extractSmartRawValue(text, attrPattern string) float64 {
	for _, line := range strings.Split(text, "\n") {
		if matched, _ := regexp.MatchString(attrPattern, line); matched {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				rawValue := fields[9]
				if idx := strings.Index(rawValue, "("); idx > 0 {
					rawValue = strings.TrimSpace(rawValue[:idx])
				}
				if v, err := strconv.ParseFloat(rawValue, 64); err == nil {
					return v
				}
			}
		}
	}
	return -1
}

func extractSmartValueColumn(text, attrPattern string) float64 {
	for _, line := range strings.Split(text, "\n") {
		if matched, _ := regexp.MatchString(attrPattern, line); matched {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				if v, err := strconv.ParseFloat(fields[3], 64); err == nil {
					return v
				}
			}
		}
	}
	return -1
}

func extractSmartInfo(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractSmartHex(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		v, err := strconv.ParseInt(strings.TrimPrefix(m[1], "0x"), 16, 64)
		if err == nil {
			return float64(v)
		}
	}
	return -1
}

func extractSmartLineLastNumber(text, prefix string) float64 {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			re := regexp.MustCompile(`(\d+)\s*$`)
			m := re.FindStringSubmatch(trimmed)
			if len(m) > 1 {
				v, _ := strconv.ParseFloat(m[1], 64)
				return v
			}
		}
	}
	return -1
}

func extractSmartSumAll(text, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return -1
	}
	var sum float64
	for _, m := range matches {
		if len(m) > 1 {
			v, _ := strconv.ParseFloat(m[1], 64)
			sum += v
		}
	}
	return sum
}

func parseSAS(d *smartDisk, out string) {
	d.HealthStatus = extractSmartInfo(out, `SMART Health Status:\s+(\w+)`)
	d.Temperature = extractSmart(out, `Current Drive Temperature:\s+(\d+)`)
	d.DriveTripTemperature = extractSmart(out, `Drive Trip Temperature:\s+(\d+)`)

	pohFloat := extractSmart(out, `number of hours powered up\s*=\s*([\d.]+)`)
	if pohFloat >= 0 {
		d.PowerOnHrs = math.Floor(pohFloat)
	}

	d.ReassignedBlocks = extractSmart(out, `Total new blocks reassigned\s*=\s*(\d+)`)
	d.GrownDefectList = extractSmart(out, `Elements in grown defect list:\s+(\d+)`)
	d.NonMediumErrors = extractSmart(out, `Non-medium error count:\s+(\d+)`)

	d.ReadUncorrectedErrors = extractSmartLineLastNumber(out, `read:`)
	d.WriteUncorrectedErrors = extractSmartLineLastNumber(out, `write:`)
	d.VerifyUncorrectedErrors = extractSmartLineLastNumber(out, `verify:`)

	d.InvalidDwordCount = extractSmartSumAll(out, `Invalid DWORD count\s*=\s*(\d+)`)
	d.RunningDisparityErrors = extractSmartSumAll(out, `Running disparity error count\s*=\s*(\d+)`)
	d.LossDwordSyncCount = extractSmartSumAll(out, `Loss of DWORD synchronization count\s*=\s*(\d+)`)
	d.PhyResetProblemCount = extractSmartSumAll(out, `Phy reset problem count\s*=\s*(\d+)`)
}

func computeHealth(d smartDisk) (health float64, status int) {
	health = 100.0

	switch {
	case d.IsNVMe:
		if d.PercentageUsed >= 0 {
			health = 100 - d.PercentageUsed
		}

		if d.CriticalWarning > 0 {
			cw := int(d.CriticalWarning)
			if cw&(1<<0) != 0 {
				health -= 30
			}
			if cw&(1<<1) != 0 {
				health -= 10
			}
			if cw&(1<<2) != 0 {
				health -= 50
			}
			if cw&(1<<3) != 0 {
				health -= 80
			}
			if cw&(1<<4) != 0 {
				health -= 30
			}
		}

		if d.AvailableSpare >= 0 && d.AvailableSpareThresh >= 0 {
			if d.AvailableSpare <= d.AvailableSpareThresh {
				health -= 30
			}
		}

		if d.MediaErrors > 0 {
			health -= d.MediaErrors * 5
		}

		if health < 0 {
			health = 0
		}

	case d.IsSAS:
		health = 100
		status = 0

		if d.HealthStatus != "" && d.HealthStatus != "OK" {
			health = 0
			status = 2
			return
		}

		if d.ReadUncorrectedErrors > 0 ||
			d.WriteUncorrectedErrors > 0 ||
			d.VerifyUncorrectedErrors > 0 {
			health = 20
			status = 2
			return
		}

		if d.NonMediumErrors > 0 {
			health = 20
			status = 2
			return
		}

		if d.ReassignedBlocks > 0 || d.GrownDefectList > 0 {
			health = 80
			status = 1
		}

		if d.Temperature >= 0 && d.DriveTripTemperature > 0 {
			if d.Temperature >= d.DriveTripTemperature-5 {
				health = min(health, 70)
				status = max(status, 1)
			}
		}

		if d.InvalidDwordCount > 0 ||
			d.RunningDisparityErrors > 0 ||
			d.LossDwordSyncCount > 100 ||
			d.PhyResetProblemCount > 0 {
			health = min(health, 80)
			status = max(status, 1)
		}

	case d.IsSSD:
		if d.AvailableResvdValue >= 0 {
			health = d.AvailableResvdValue
		} else if d.WearLevelingValue >= 0 {
			health = d.WearLevelingValue
		}
		if d.ReallocatedSectors > 0 {
			health -= d.ReallocatedSectors * 2
		}
		if d.PendingSectors > 0 {
			health -= d.PendingSectors * 3
		}
		if d.OfflineUncorrectable > 0 {
			health -= d.OfflineUncorrectable * 5
		}
		if d.ProgramFailCntTotal > 0 {
			health -= d.ProgramFailCntTotal * 3
		}
		if d.EraseFailCountTotal > 0 {
			health -= d.EraseFailCountTotal * 2
		}
		if health < 0 {
			health = 0
		}

	default:
		// 1. CRITICAL GROUP
		// Reallocated Sectors: log scale, cap at 50%
		if d.ReallocatedSectors > 0 {
			penalty := 5.0 + math.Log2(d.ReallocatedSectors+1)*4.0
			if penalty > 50 {
				penalty = 50
			}
			health -= penalty
		}

		// Pending Sectors & Offline Uncorrectable usually report the same
		// failing sectors — take the worst to avoid double-counting.
		badSectors := math.Max(d.PendingSectors, d.OfflineUncorrectable)
		if badSectors > 0 {
			penalty := 10.0 + math.Log2(badSectors+1)*5.0
			if penalty > 80 {
				penalty = 80
			}
			health -= penalty
		}

		// Reported Uncorrectable: log scale, cap at 25%
		if d.ReportedUncorrect > 0 {
			penalty := 5.0 + math.Log2(d.ReportedUncorrect+1)*3.0
			if penalty > 25 {
				penalty = 25
			}
			health -= penalty
		}

		if d.RuntimeBadBlock > 0 {
			penalty := d.RuntimeBadBlock * 4.0
			if penalty > 40 {
				penalty = 40
			}
			health -= penalty
		}
		if d.EndToEndErrors > 0 {
			health -= d.EndToEndErrors * 3.0
		}

		// 2. MECHANICAL GROUP
		if d.SpinRetryCount > 0 {
			penalty := 20.0 + d.SpinRetryCount*5.0
			if penalty > 45 {
				penalty = 45
			}
			health -= penalty
		}

		// 3. AGE & WEAR GROUP
		if d.PowerOnHrs > 43800 {
			health -= 10.0
		} else if d.PowerOnHrs > 26280 {
			health -= 5.0
		} else if d.PowerOnHrs > 17520 {
			health -= 2.0
		}

		// 4. POWER & STABILITY
		if d.PowerOnHrs > 0 && d.PowerCycles > 0 {
			cyclesPerDay := d.PowerCycles / (d.PowerOnHrs / 24.0)
			if cyclesPerDay > 5 {
				health -= 10.0
			} else if cyclesPerDay > 3 {
				health -= 5.0
			}
		}
		if d.PowerOffRetract > 1000 {
			health -= 5.0
		} else if d.PowerOffRetract > 500 {
			health -= 2.0
		}

		// 5. CONNECTION
		if d.UDMACRCErrors > 0 {
			penalty := d.UDMACRCErrors * 0.3
			if penalty > 15 {
				penalty = 15
			}
			health -= penalty
		}

		// 6. TEMPERATURE
		if d.Temperature > 0 {
			if d.Temperature > 65 {
				health -= 20.0
			} else if d.Temperature > 55 {
				health -= 10.0
			} else if d.Temperature > 50 {
				health -= 5.0
			}
			if d.Temperature < 20 {
				health -= 5.0
			}
		}

		if health < 0 {
			health = 0
		}
	}

	if health < 50 {
		status = 2 // CRITICAL
	} else if health < 80 {
		status = 1 // WARNING
	}
	return
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func sanitizeLabel(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `"`, `'`)
}

func (e *promExporter) readSMART(dev smartDevice) (smartDisk, bool) {
	runSmartctl := func(args ...string) (string, error) {
		cmdArgs := make([]string, 0, len(args)+3)
		cmdArgs = append(cmdArgs, args...)
		if dev.Driver != "" {
			cmdArgs = append(cmdArgs, "-d", dev.Driver)
		}
		cmdArgs = append(cmdArgs, dev.Path)
		cmd := exec.Command(e.smartctlPath, cmdArgs...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	info, _ := runSmartctl("-i")
	isSAS := strings.Contains(info, "Transport protocol:   SAS") || strings.Contains(info, "Vendor:")

	var out string
	if isSAS {
		xOut, err := runSmartctl("-x")
		if err != nil && len(xOut) == 0 {
			return smartDisk{}, false
		}
		out = string(xOut)
		if !isSAS {
			isSAS = strings.Contains(out, "SMART Health Status")
		}
	} else {
		aOut, err := runSmartctl("-A")
		if err != nil && len(aOut) == 0 {
			return smartDisk{}, false
		}
		out = string(aOut)
	}

	isNVMe := !isSAS && strings.Contains(out, "NVMe")
	isSATASSD := !isNVMe && !isSAS && isSolidState(info)

	d := initSmartDisk()
	d.Device = dev.Path
	d.Driver = dev.Driver
	d.Model = extractSmartInfo(info, `(?:Device Model|Model Number|Product):\s+(.+)`)
	d.Serial = extractSmartInfo(info, `(?i)Serial Number:\s+(.+)`)
	d.WWN = extractSmartInfo(info, `(?:LU WWN Device Id|Logical Unit id):\s+(.+)`)
	d.Firmware = extractSmartInfo(info, `(?:Firmware Version|Revision):\s+(.+)`)
	d.IsNVMe = isNVMe
	d.IsSSD = isNVMe || isSATASSD
	d.IsSAS = isSAS

	identifier := d.Serial
	if identifier == "" {
		identifier = d.WWN
	}
	if identifier == "" && d.Model != "" && d.Driver != "" {
		identifier = d.Model + "_" + d.Driver
	}
	if identifier == "" {
		identifier = d.Device
		if d.Driver != "" {
			identifier += "_" + d.Driver
		}
	}
	d.Name = strings.ReplaceAll(identifier, " ", "_")

	if isNVMe {
		fullText := info + "\n" + out
		d.Temperature = extractSmart(out, `Temperature:\s+(\d+)\s+Celsius`)
		d.CriticalWarning = extractSmartHex(out, `Critical Warning:\s+(0x[0-9a-fA-F]+)`)
		d.PowerOnHrs = extractSmartComma(out, `Power On Hours:\s+([\d,]+)`)
		d.PercentageUsed = extractSmart(out, `Percentage Used:\s+(\d+)%`)
		d.AvailableSpare = extractSmart(out, `Available Spare:\s+(\d+)%`)
		d.AvailableSpareThresh = extractSmart(out, `Available Spare Threshold:\s+(\d+)%`)
		d.DataUnitsRead = extractSmartComma(out, `Data Units Read:\s+([\d,]+)`)
		d.DataUnitsWritten = extractSmartComma(out, `Data Units Written:\s+([\d,]+)`)
		d.HostReadCommands = extractSmartComma(out, `Host Read Commands:\s+([\d,]+)`)
		d.HostWriteCommands = extractSmartComma(out, `Host Write Commands:\s+([\d,]+)`)
		d.ControllerBusyTime = extractSmartComma(out, `Controller Busy Time:\s+([\d,]+)`)
		d.PowerCycles = extractSmartComma(out, `Power Cycles:\s+([\d,]+)`)
		d.UnsafeShutdowns = extractSmartComma(out, `Unsafe Shutdowns:\s+([\d,]+)`)
		d.MediaErrors = extractSmartComma(out, `Media and Data Integrity Errors:\s+([\d,]+)`)
		d.ErrorLogEntries = extractSmartComma(out, `Error Information Log Entries:\s+([\d,]+)`)
		d.WarningTempTime = extractSmartComma(out, `Warning\s+Comp\.\s+Temperature Time:\s+([\d,]+)`)
		d.CriticalTempTime = extractSmartComma(out, `Critical\s+Comp\.\s+Temperature Time:\s+([\d,]+)`)
		d.WarningTempThresh = extractSmart(fullText, `Warning\s+Comp\.\s+Temp\.\s+Threshold:\s+(\d+)\s+Celsius`)
		d.CriticalTempThresh = extractSmart(fullText, `Critical\s+Comp\.\s+Temp\.\s+Threshold:\s+(\d+)\s+Celsius`)
		d.TempSensor1 = extractSmart(out, `Temperature Sensor 1:\s+(\d+)\s+Celsius`)
		d.TempSensor2 = extractSmart(out, `Temperature Sensor 2:\s+(\d+)\s+Celsius`)
	} else if isSAS {
		parseSAS(&d, out)
	} else {
		d.Temperature = extractSmartRawValue(out, `194\s+Temperature_Celsius`)
		if d.Temperature < 0 {
			d.Temperature = extractSmartRawValue(out, `190\s+Airflow_Temperature_Cel`)
		}
		d.PowerOnHrs = extractSmartRawValue(out, `9\s+Power_On_Hours`)
		d.PowerCycles = extractSmartRawValue(out, `12\s+Power_Cycle_Count`)
		d.RawReadErrorRate = extractSmartRawValue(out, `1\s+Raw_Read_Error_Rate`)
		d.ReallocatedSectors = extractSmartRawValue(out, `5\s+Reallocated_Sector_Ct`)
		d.PendingSectors = extractSmartRawValue(out, `197\s+Current_Pending_Sector`)
		d.OfflineUncorrectable = extractSmartRawValue(out, `198\s+Offline_Uncorrectable`)
		d.UDMACRCErrors = extractSmartRawValue(out, `199\s+UDMA_CRC_Error_Count`)
		d.RuntimeBadBlock = extractSmartRawValue(out, `183\s+Runtime_Bad_Block`)
		d.ReportedUncorrect = extractSmartRawValue(out, `187\s+Reported_Uncorrect`)
		d.EndToEndErrors = extractSmartRawValue(out, `184\s+End-to-End_Error`)
		d.PowerOffRetract = extractSmartRawValue(out, `192\s+Power-Off_Retract_Count`)

		if isSATASSD {
			d.WearLevelingValue = extractSmartValueColumn(out, `177\s+Wear_Leveling_Count`)
			d.WearLevelingRaw = extractSmartRawValue(out, `177\s+Wear_Leveling_Count`)
			d.AvailableResvdValue = extractSmartValueColumn(out, `232\s+Available_Reservd_Space`)
			d.TotalLBAsWritten = extractSmartRawValue(out, `241\s+Total_LBAs_Written`)
			d.TotalLBAsRead = extractSmartRawValue(out, `242\s+Total_LBAs_Read`)
			d.EraseFailCountChip = extractSmartRawValue(out, `176\s+Erase_Fail_Count_Chip`)
			d.EraseFailCountTotal = extractSmartRawValue(out, `182\s+Erase_Fail_Count_Total`)
			d.ProgramFailCntTotal = extractSmartRawValue(out, `181\s+Program_Fail_Cnt_Total`)
			d.UsedRsvdBlkCntChip = extractSmartRawValue(out, `178\s+Used_Rsvd_Blk_Cnt_Chip`)
		} else {
			d.SpinRetryCount = extractSmartRawValue(out, `10\s+Spin_Retry_Count`)
		}
	}

	d.Health, d.Status = computeHealth(d)
	return d, true
}

func (e *promExporter) getSmartMetrics() ([]metric, error) {
	if e.smartctlPath == "" {
		return nil, nil
	}

	cmd := exec.Command(e.smartctlPath, "--scan-open")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		e.Logger.Printf("smartctl --scan-open failed: %v", err)
		return nil, nil
	}

	var devices []smartDevice
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "#", 2)
		fields := strings.Fields(parts[0])
		if len(fields) > 0 {
			dev := smartDevice{Path: fields[0]}
			for i := 1; i < len(fields)-1; i++ {
				if fields[i] == "-d" {
					dev.Driver = fields[i+1]
					break
				}
			}

			if dev.Driver == "scsi" {
				probeCmd := exec.Command(e.smartctlPath, "-i", "-d", "sat", dev.Path)
				probeOut, _ := probeCmd.CombinedOutput()
				probeStr := string(probeOut)
				if strings.Contains(probeStr, "SATA Version is:") || strings.Contains(probeStr, "ATA Version is:") {
					dev.Driver = "sat"
				}
			}

			devices = append(devices, dev)
		}
	}

	var disks []smartDisk
	for _, dev := range devices {
		if d, ok := e.readSMART(dev); ok {
			disks = append(disks, d)
		}
	}

	if len(disks) == 0 {
		return nil, nil
	}

	var metrics []metric

	for _, d := range disks {
		diskType := "hdd"
		if d.IsNVMe {
			diskType = "nvme"
		} else if d.IsSAS {
			diskType = "sas"
		} else if d.IsSSD {
			diskType = "ssd"
		}

		model := sanitizeLabel(d.Model)
		serial := sanitizeLabel(d.Serial)
		firmware := sanitizeLabel(d.Firmware)
		device := sanitizeLabel(d.Device)
		driver := sanitizeLabel(d.Driver)

		attr := fmt.Sprintf(`disk=%q,device=%q,driver=%q,type=%q,model=%q,serial=%q,firmware=%q`,
			d.Name, device, driver, diskType, model, serial, firmware)
		attrShort := fmt.Sprintf(`disk=%q,device=%q,driver=%q,model=%q,serial=%q`,
			d.Name, device, driver, model, serial)

		metrics = append(metrics, metric{
			name:       "smart_disk_health_percent",
			attr:       attr,
			value:      d.Health,
			help:       "Disk health percentage (gauge)",
			metricType: "gauge",
		})
		metrics = append(metrics, metric{
			name:       "smart_disk_status",
			attr:       attrShort,
			value:      float64(d.Status),
			help:       "Disk status 0=OK 1=WARNING 2=CRITICAL (gauge)",
			metricType: "gauge",
		})

		addMetric := func(metricName string, val float64, help string, metricType string) {
			if val >= 0 {
				metrics = append(metrics, metric{
					name:       metricName,
					attr:       attrShort,
					value:      val,
					help:       help,
					metricType: metricType,
				})
			}
		}

		addMetric("smart_disk_temperature_celsius", d.Temperature, "Disk temperature in Celsius (gauge)", "gauge")
		addMetric("smart_disk_power_on_hours", d.PowerOnHrs, "Disk power on hours (gauge)", "gauge")
		addMetric("smart_disk_power_cycles_total", d.PowerCycles, "Power cycle count (counter)", "counter")
		addMetric("smart_disk_power_off_retract_count", d.PowerOffRetract, "Power-off retract count (counter)", "counter")

		// NVMe
		addMetric("smart_disk_critical_warning", d.CriticalWarning, "Critical warning bitmask NVMe 0=OK (gauge)", "gauge")
		addMetric("smart_disk_available_spare_percent", d.AvailableSpare, "Available spare percentage NVMe (gauge)", "gauge")
		addMetric("smart_disk_available_spare_threshold_percent", d.AvailableSpareThresh, "Available spare threshold NVMe (gauge)", "gauge")
		addMetric("smart_disk_percentage_used_percent", d.PercentageUsed, "Percentage used NVMe (gauge)", "gauge")
		addMetric("smart_disk_data_units_read_total", d.DataUnitsRead, "Data units read NVMe (counter)", "counter")
		addMetric("smart_disk_data_units_written_total", d.DataUnitsWritten, "Data units written NVMe (counter)", "counter")
		addMetric("smart_disk_host_read_commands_total", d.HostReadCommands, "Host read commands NVMe (counter)", "counter")
		addMetric("smart_disk_host_write_commands_total", d.HostWriteCommands, "Host write commands NVMe (counter)", "counter")
		addMetric("smart_disk_controller_busy_time", d.ControllerBusyTime, "Controller busy time NVMe (gauge)", "gauge")
		addMetric("smart_disk_unsafe_shutdowns_total", d.UnsafeShutdowns, "Unsafe shutdowns NVMe (counter)", "counter")
		addMetric("smart_disk_media_errors_total", d.MediaErrors, "Media and data integrity errors NVMe (counter)", "counter")
		addMetric("smart_disk_error_log_entries_total", d.ErrorLogEntries, "Error information log entries NVMe (counter)", "counter")
		addMetric("smart_disk_warning_temperature_time_minutes_total", d.WarningTempTime, "Warning temperature time minutes NVMe (counter)", "counter")
		addMetric("smart_disk_critical_temperature_time_minutes_total", d.CriticalTempTime, "Critical temperature time minutes NVMe (counter)", "counter")
		addMetric("smart_disk_warning_temperature_threshold_celsius", d.WarningTempThresh, "Warning temperature threshold NVMe (gauge)", "gauge")
		addMetric("smart_disk_critical_temperature_threshold_celsius", d.CriticalTempThresh, "Critical temperature threshold NVMe (gauge)", "gauge")
		addMetric("smart_disk_temperature_sensor1_celsius", d.TempSensor1, "Temperature sensor 1 NVMe (gauge)", "gauge")
		addMetric("smart_disk_temperature_sensor2_celsius", d.TempSensor2, "Temperature sensor 2 NVMe (gauge)", "gauge")

		// SATA SSD
		addMetric("smart_disk_wear_leveling_value", d.WearLevelingValue, "Wear leveling VALUE score SATA SSD 100=new 0=worn (gauge)", "gauge")
		addMetric("smart_disk_wear_leveling_raw", d.WearLevelingRaw, "Wear leveling RAW erase cycle count SATA SSD (gauge)", "gauge")
		addMetric("smart_disk_available_reservd_space_value", d.AvailableResvdValue, "Available reserved space VALUE score SATA SSD 100=full (gauge)", "gauge")
		addMetric("smart_disk_total_lbas_written_total", d.TotalLBAsWritten, "Total LBAs written SATA SSD (counter)", "counter")
		addMetric("smart_disk_total_lbas_read_total", d.TotalLBAsRead, "Total LBAs read SATA SSD (counter)", "counter")
		addMetric("smart_disk_erase_fail_count_chip_total", d.EraseFailCountChip, "Erase fail count chip SATA SSD (counter)", "counter")
		addMetric("smart_disk_erase_fail_count_total_total", d.EraseFailCountTotal, "Erase fail count total SATA SSD (counter)", "counter")
		addMetric("smart_disk_program_fail_cnt_total_total", d.ProgramFailCntTotal, "Program fail count total SATA SSD (counter)", "counter")
		addMetric("smart_disk_used_rsvd_blk_cnt_chip", d.UsedRsvdBlkCntChip, "Used reserved block count chip SATA SSD (gauge)", "gauge")

		// Common SATA
		addMetric("smart_disk_raw_read_error_rate", d.RawReadErrorRate, "Raw read error rate (gauge)", "gauge")
		addMetric("smart_disk_reallocated_sectors_total", d.ReallocatedSectors, "Reallocated sectors count (counter)", "counter")
		addMetric("smart_disk_pending_sectors", d.PendingSectors, "Current pending sectors (gauge)", "gauge")
		addMetric("smart_disk_offline_uncorrectable", d.OfflineUncorrectable, "Offline uncorrectable sectors (gauge)", "gauge")
		addMetric("smart_disk_udma_crc_errors_total", d.UDMACRCErrors, "UDMA CRC error count (counter)", "counter")
		addMetric("smart_disk_runtime_bad_block", d.RuntimeBadBlock, "Runtime bad block count (gauge)", "gauge")
		addMetric("smart_disk_reported_uncorrect_total", d.ReportedUncorrect, "Reported uncorrect errors (counter)", "counter")
		addMetric("smart_disk_end_to_end_errors_total", d.EndToEndErrors, "End-to-end errors (counter)", "counter")

		// HDD
		addMetric("smart_disk_spin_retry_count_total", d.SpinRetryCount, "Spin retry count HDD (counter)", "counter")

		// SAS
		addMetric("smart_disk_trip_temperature_celsius", d.DriveTripTemperature, "Drive trip temperature SAS (gauge)", "gauge")
		addMetric("smart_disk_reassigned_blocks", d.ReassignedBlocks, "Total new blocks reassigned SAS (gauge)", "gauge")
		addMetric("smart_disk_grown_defect_list", d.GrownDefectList, "Elements in grown defect list SAS (gauge)", "gauge")
		addMetric("smart_disk_non_medium_errors", d.NonMediumErrors, "Non-medium error count SAS (gauge)", "gauge")
		addMetric("smart_disk_read_uncorrected_errors", d.ReadUncorrectedErrors, "Read uncorrected errors SAS (gauge)", "gauge")
		addMetric("smart_disk_write_uncorrected_errors", d.WriteUncorrectedErrors, "Write uncorrected errors SAS (gauge)", "gauge")
		addMetric("smart_disk_verify_uncorrected_errors", d.VerifyUncorrectedErrors, "Verify uncorrected errors SAS (gauge)", "gauge")
		addMetric("smart_disk_invalid_dword_count", d.InvalidDwordCount, "Invalid DWORD count SAS (gauge)", "gauge")
		addMetric("smart_disk_running_disparity_errors", d.RunningDisparityErrors, "Running disparity error count SAS (gauge)", "gauge")
		addMetric("smart_disk_loss_dword_sync_count", d.LossDwordSyncCount, "Loss of DWORD synchronization count SAS (gauge)", "gauge")
		addMetric("smart_disk_phy_reset_problem_count", d.PhyResetProblemCount, "Phy reset problem count SAS (gauge)", "gauge")
	}

	return metrics, nil
}
