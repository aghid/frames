# Copyright 2018 Iguazio
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# Exract min version ($$ is Makefile escape)
go_minver = $(shell go version | awk '{print $$3}' | awk -F. '{print $$2}')
ifeq ($(go_minver),11)
    modflag = -mod=vendor
endif

all:
	@echo Please pick a target
	@egrep '^[^ :]+:' Makefile | \
	   grep -v all | \
	   sed -e 's/://' -e 's/^/    /' | \
	   sort
	@false

test:
	GO111MODULE=on go test -v $(testflags) $(modflag) ./...

build:
	GO111MODULE=on go build -v $(modflag) ./...

test-python:
	cd clients/py && \
	   PIPENV_IGNORE_VIRTUALENVS=1 \
	   pipenv run flake8 --exclude 'frames_pb2*.py' v3io_frames tests
	cd clients/py && \
	    PIPENV_IGNORE_VIRTUALENVS=1 \
	    pipenv run python -m pytest -v --disable-warnings

build-docker:
	docker build -f ./cmd/framesd/Dockerfile -t v3io/framesd .

wheel:
	cd clients/py && python setup.py bdist_wheel

update-tsdb-dep:
	GO111MODULE=on go get github.com/v3io/v3io-tsdb@development
	GO111MODULE=on go mod vendor
	@echo "Done. Don't forget to commit ☺"

grpc: grpc-go grpc-py

grpc-go:
	protoc  frames.proto --go_out=plugins=grpc:pb

grpc-py:
	cd clients/py && \
	pipenv run python -m grpc_tools.protoc \
		-I../.. --python_out=v3io_frames\
		--grpc_python_out=v3io_frames \
		../../frames.proto
	python scripts/fix_pb_import.py \
	    clients/py/v3io_frames/frames_pb2_grpc.py

travis-py:
	curl -LO https://dl.google.com/go/go1.11.1.linux-amd64.tar.gz
	tar xzf go1.11.1.linux-amd64.tar.gz -C /opt
	pip install pipenv
	cd clients/py && pipenv sync --dev
	cd clients/py && \
	    PATH=/opt/go:$(PATH) pipenv run python -m pytest -v
