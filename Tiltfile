load('ext://restart_process', 'docker_build_with_restart')

CONTROLLER_DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/manager /
CMD ["/manager"]
'''

# Generate manifests and go files
local_resource('make manifests', "make manifests", deps=["api", "controllers", "hooks"], ignore=['*/*/zz_generated.deepcopy.go'])
local_resource('make generate', "make generate", deps=["api", "hooks"], ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy CRD
local_resource(
    'CRD', 'make install', deps=["api"],
    ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy manager
watch_file('./config/')
k8s_yaml(kustomize('./config/dev'))

local_resource(
    'Watch & Compile', "make build", deps=['controllers', 'api', 'hooks', 'pkg', 'main.go'],
    ignore=['*/*/zz_generated.deepcopy.go'])

docker_build_with_restart(
    'necotiator:dev', '.',
    dockerfile_contents=CONTROLLER_DOCKERFILE,
    entrypoint=['/manager'],
    only=['./bin/manager'],
    live_update=[
        sync('./bin/manager', '/manager'),
    ]
)

local_resource(
    'Sample', 'kubectl apply -f ./config/samples/necotiator_v1beta1_tenantresourcequota.yaml',
    deps=["./config/samples/necotiator_v1beta1_tenantresourcequota.yaml"])
