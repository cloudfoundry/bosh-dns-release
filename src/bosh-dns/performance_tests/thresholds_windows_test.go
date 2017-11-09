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
		Pct90: 17 * time.Millisecond,
		Pct95: 20 * time.Millisecond,
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
		CPUPct99: 30,
		MemPct99: 36,
		MemMax:   40,
	}
}

func localZonesTimeThresholds() TimeThresholds {
	return TimeThresholds{
		Med:   3 * time.Millisecond,
		Pct90: 4 * time.Millisecond,
		Pct95: 5 * time.Millisecond,
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
