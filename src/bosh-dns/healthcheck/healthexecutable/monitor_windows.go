package healthexecutable

func (m *Monitor) runExecutable(executable string) (stdout, stderr string, exitStatus int, err error) {
	return m.cmdRunner.RunCommand("powershell.exe", executable)
}
