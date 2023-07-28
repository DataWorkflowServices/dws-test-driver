# dws-test-driver
A Data Workflow Services (DWS) driver implementation used for integration testing.

See `config/samples/base-workflow.yaml` for a basic example for `Proposal` state.

See `config/samples/complex-workflow.yaml` and its accompanying script `config/samples/run-complex.sh` for a more complex example showing a workflow that pauses in `Setup` state for the test script and then transitions to `DataIn` state where it stops with a specified error message.


