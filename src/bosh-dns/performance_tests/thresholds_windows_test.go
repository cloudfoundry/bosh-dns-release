package performance_test

import "time"

func healthVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 73,
		MemPct99: 20,
		MemMax:   25,
	}
}

func healthTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   16 * time.Millisecond,
		Pct90: 20 * time.Millisecond,
		Pct95: 25 * time.Millisecond,
	}
}

func prodLikeVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 20,
		MemPct99: 18,
		MemMax:   20,
	}
}

func localZonesVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 35,
		MemPct99: 36,
		MemMax:   40,
	}
}

func localZonesTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   6 * time.Millisecond,
		Pct90: 8 * time.Millisecond,
		Pct95: 10 * time.Millisecond,
	}
}

func upcheckVitalsThresholds() VitalsThresholds {
	return VitalsThresholds{
		CPUPct99: 10,
		MemPct99: 17,
		MemMax:   20,
	}
}

func upcheckTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   2 * time.Millisecond,
		Pct90: 3 * time.Millisecond,
		Pct95: 4 * time.Millisecond,
	}
}
