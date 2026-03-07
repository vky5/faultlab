.PHONY: node1 node2 node3

NODE_CMD := go run ./cmd/node

node1:
	$(NODE_CMD) -id node1 -port 7001 --peers node2:7002,node3:7003

node2:
	$(NODE_CMD) -id node2 -port 7002 --peers node1:7001,node3:7003

node3:
	$(NODE_CMD) -id node3 -port 7003 --peers node1:7001,node2:7002
