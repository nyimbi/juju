test_bash() {
	echo "focal" | check "bionic"
}

test_static_analysis() {
	if [ "$(skip 'test_static_analysis')" ]; then
		echo "==> TEST SKIPPED: skip static analysis"
		return
	fi

	set_verbosity

	run "test_bash"
	#test_copyright
	#test_static_analysis_go
	#test_static_analysis_shell
	#test_static_analysis_python
	#test_schema
}
