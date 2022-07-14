#!/bin/bash
NAMESPACE=${NAMESPACE:-"default"}
SERVER_NAMESPACE=${SERVER_NAMESPACE:-${NAMESPACE}}
REQUESTER_NAMESPACE=${REQUESTER_NAMESPACE:-${NAMESPACE}}
REPLICAS=${REPLICAS:-"3"}
SERVER_NAME=${SERVER_NAME:-"serve"}
REQUESTER_NAME=${REQUESTER_NAME:-"request"}
TAG=${TAG:-"latest"}
RESOURCES=${RESOURCES:-"{\"cpu\":100m}"}
NODE_SELECTOR=${NODE_SELECTOR:-"{}"}
SERVER_NODE_SELECTOR=${SERVER_NODE_SELECTOR:-"${NODE_SELECTOR}"}
REQUESTER_NODE_SELECTOR=${REQUESTER_NODE_SELECTOR:-"${NODE_SELECTOR}"}

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${SERVER_NAMESPACE}
---
apiVersion: v1
kind: Namespace
metadata:
  name: ${REQUESTER_NAMESPACE}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: ${SERVER_NAMESPACE}
  name: ${SERVER_NAME}
spec:
  selector:
    matchLabels:
      app: ${SERVER_NAME}
  replicas: ${REPLICAS}
  template:
    metadata:
      labels:
        app: ${SERVER_NAME}
    spec:
      nodeSelector: ${SERVER_NODE_SELECTOR}
      containers:
      - env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        image: jeremyot/serve:${TAG}
        args:
        - --http=:80
        - --message=Hello from {{env "POD_NAME"}} ({{addr}}) on node {{env "NODE_NAME"}}
        name: ${SERVER_NAME}
        resources:
          requests: ${RESOURCES}
---
apiVersion: v1
kind: Service
metadata:
  namespace: ${SERVER_NAMESPACE}
  name: ${SERVER_NAME}
spec:
  selector:
    app: ${SERVER_NAME}
  ports:
  - name: http
    port: 80
    protocol: TCP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: ${REQUESTER_NAMESPACE}
  name: ${REQUESTER_NAME}
spec:
  selector:
    matchLabels:
      app: ${REQUESTER_NAME}
  replicas: ${REPLICAS}
  template:
    metadata:
      labels:
        app: ${REQUESTER_NAME}
    spec:
      nodeSelector: ${REQUESTER_NODE_SELECTOR}
      containers:
      - image: jeremyot/request:${TAG}
        args:
        - --address=http://${SERVER_NAME}.${SERVER_NAMESPACE}.svc.cluster.local
        - --latency
        name: ${REQUESTER_NAME}-2
        resources:
          requests: ${RESOURCES}
EOF