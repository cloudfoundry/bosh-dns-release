// +build !windows

package healthexecutable

func (m *HealthExecutableMonitor) runExecutable(executable string) (stdout, stderr string, exitStatus int, err error) {
	return m.cmdRunner.RunCommand(executable)
}
