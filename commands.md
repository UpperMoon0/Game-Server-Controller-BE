Remove all node containers and volumes
docker rm -f $(docker ps -a --filter "name=^game-server-node-" -q) && docker volume rm -f $(docker volume ls -q | grep '^game-server-node-')
