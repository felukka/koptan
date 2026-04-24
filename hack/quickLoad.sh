#!/usr/bin/env bash

kubectl apply -f examples/goapp.yml
kubectl apply -f examples/java.yml
kubectl apply -f examples/dotnet.yml
sleep 5
kubectl get goapp,javaapp,dotnetapp,slipway,voyage
