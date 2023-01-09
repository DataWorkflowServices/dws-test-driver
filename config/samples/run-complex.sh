#!/bin/bash

# This script simulates the behavior of a DWS driver, which would normally be
# written as a k8s controller.

set -ex

WF_YAML=config/samples/complex-workflow.yaml

# Apply the workflow.
kubectl apply -f $WF_YAML
kubectl wait workflow --timeout=120s -n default complex --for jsonpath='{.status.status}'=Completed

# Request that the workflow begin its transition to Setup state.  Our workflow
# directive will leave it with a status of DriverWait.
kubectl patch workflow complex --type=merge -p '{"spec":{"desiredState":"Setup"}}'
kubectl wait workflow --timeout=120s -n default complex --for jsonpath='{.status.status}'=DriverWait

# Mark the individual Setup action with a status of Completed.
kubectl patch workflow complex --type=json -p '[{"op":"replace", "path":"/status/drivers/0/status", "value": "Completed"}, {"op":"replace", "path":"/status/drivers/0/completed", "value": true}]'

# Watch the workflow's overall status go to Completed.
kubectl wait workflow --timeout=120s -n default complex --for jsonpath='{.status.status}'=Completed

# Request that workflow transition to DataIn.  Our workflow directive will
# leave it in an error state.
kubectl patch workflow complex --type=merge -p '{"spec":{"desiredState":"DataIn"}}'
kubectl wait workflow --timeout=120s -n default complex --for jsonpath='{.status.status}'=Error

# Move the workflow to Teardown.
kubectl patch workflow complex --type=merge -p '{"spec":{"desiredState":"Teardown"}}'
kubectl wait workflow --timeout=120s -n default complex --for jsonpath='{.status.status}'=Completed

# Delete the workflow.
kubectl delete -f $WF_YAML

