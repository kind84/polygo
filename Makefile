all: build run

build-dependencies:
	docker build -t dependencies -f ./dependencies.Dockerfile .

build: build-dependencies
	docker-compose build

run:
	docker-compose up
