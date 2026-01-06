# Run
```shell
kubectl -n tg-stats create secret generic env-secret --from-env-file=./.env
kubectl apply -f namespace/
kubectl apply -f config/
kubectl apply -f pvc/
kubectl apply -f service/
kubectl apply -f deployments/
```