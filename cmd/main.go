package cmd

func Run(args []string) error {
	maybeStartInvocationUpdateCheck(args)
	cmd := newRootCommand(args)
	return cmd.Execute()
}
