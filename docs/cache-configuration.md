# Cache Configuration for Tackle Hub

This document describes how the cache storage is configured in the Koncur test environment for Tackle Hub.

## Overview

The cache configuration ensures that Tackle Hub's cache data persists across Kind cluster recreations, improving test iteration speed and reducing redundant dependency downloads.

## Architecture

The cache system uses a three-layer approach:

1. **Host Directory**: `./cache/hub-cache` on your local machine
2. **Kind Node Mount**: Mounted to `/cache` inside the Kind control-plane node
3. **Kubernetes PersistentVolume**: Pre-created PV that uses the node's `/cache/hub-cache` directory

## Configuration Components

### 1. Kind Cluster Configuration

The Kind cluster is configured with an extra mount that maps the local `./cache` directory to `/cache` inside the Kind node:

```yaml
extraMounts:
  - hostPath: ./cache
    containerPath: /cache
```

This is defined in `.koncur/config/kind-config.yaml` and is created by the `make kind-create` target.

**Location in Makefile**: Lines 36-38

### 2. Local-Path Storage Provisioner

The local-path-storage provisioner is configured to support ReadWriteMany (RWX) access mode using a shared filesystem path:

```json
{
  "nodePathMap": [],
  "sharedFileSystemPath": "/cache"
}
```

This configuration:
- Enables RWX access mode support (required by Tackle Hub cache PVC)
- Uses `/cache` as the shared directory for all PVCs
- Removes the default nodePathMap to prevent RWO-only provisioning

**Location in Makefile**: Lines 40-44

### 3. Pre-created PersistentVolume

A static PersistentVolume is created before Tackle Hub installation:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: tackle-cache-pv
  labels:
    type: tackle-cache
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: manual
  hostPath:
    path: /cache/hub-cache
    type: DirectoryOrCreate
```

Key properties:
- **storageClassName**: `manual` - Prevents dynamic provisioning
- **accessModes**: `ReadWriteMany` - Allows multiple pods to access
- **persistentVolumeReclaimPolicy**: `Retain` - Keeps data when PVC is deleted
- **hostPath**: `/cache/hub-cache` - Fixed directory path

**Location in Makefile**: Lines 93-112

### 4. Tackle CR Configuration

The Tackle Custom Resource is configured to use the manual storage class:

```yaml
spec:
  cache_storage_class: "manual"
  cache_data_volume_size: "10Gi"
  rwx_supported: "true"
```

This ensures that the Tackle operator creates a PVC that binds to our pre-created PV.

**Location in Makefile**: Lines 120-124

## Directory Structure

```
koncur/
├── cache/                          # Host directory (gitignored)
│   └── hub-cache/                  # Tackle Hub cache data
│       └── ...                     # Maven dependencies, etc.
├── .koncur/
│   └── config/
│       ├── kind-config.yaml        # Kind cluster config with mount
│       ├── cache-pv.yaml           # Pre-created PV manifest
│       └── tackle-cr.yaml          # Tackle CR with cache config
└── Makefile                        # Automation scripts
```

## Lifecycle

### Cluster Creation (`make kind-create`)

1. Create `./cache` directory
2. Generate Kind config with cache mount
3. Create Kind cluster
4. Configure local-path-storage for RWX support
5. Install ingress-nginx

### Tackle Installation (`make hub-install`)

1. Install OLM
2. Install Tackle operator
3. Create cache directory: `./cache/hub-cache`
4. Create PersistentVolume with fixed path
5. Create Tackle CR with manual storage class
6. Wait for Tackle Hub to be ready

### Cluster Deletion (`make kind-delete`)

The cache data persists in `./cache/hub-cache` even after cluster deletion, allowing for faster subsequent installations.

## Benefits

1. **Faster Test Iterations**: Dependencies are cached across cluster recreations
2. **Reduced Network Usage**: No need to re-download Maven dependencies
3. **Consistent Test Environment**: Same cache data across test runs
4. **Easy Cleanup**: Delete `./cache` directory to start fresh

## Troubleshooting

### PVC Not Binding to Pre-created PV

Check that the storage class matches:

```bash
kubectl get pv tackle-cache-pv -o jsonpath='{.spec.storageClassName}'
kubectl get pvc -n konveyor-tackle tackle-cache-volume-claim -o jsonpath='{.spec.storageClassName}'
```

Both should return `manual`.

### Cache Not Persisting

Verify the mount is working:

```bash
# Check if the directory exists in the Kind node
docker exec koncur-test-control-plane ls -la /cache/hub-cache

# Check if the PV is using the correct path
kubectl get pv tackle-cache-pv -o jsonpath='{.spec.hostPath.path}'
```

### RWX Access Mode Not Supported

Verify local-path-storage configuration:

```bash
kubectl get configmap local-path-config -n local-path-storage -o yaml
```

Should contain `sharedFileSystemPath: "/cache"`.

## Configuration Reference

### Storage Class: `manual`

Used for pre-provisioned volumes. The Tackle operator will create a PVC with this storage class, which binds to our pre-created PV.

### Storage Class: `standard` (default)

The default storage class used by local-path-provisioner. We avoid this because it provisions new PVs dynamically, preventing us from using our pre-created PV with a fixed path.

### Access Mode: `ReadWriteMany` (RWX)

Required by Tackle Hub because multiple pods may need access to the cache simultaneously. Local-path-provisioner only supports RWX when configured with `sharedFileSystemPath`.

## See Also

- [Tackle Operator Documentation](https://github.com/konveyor/tackle2-operator)
- [Kind Extra Mounts](https://kind.sigs.k8s.io/docs/user/configuration/#extra-mounts)
- [Local Path Provisioner](https://github.com/rancher/local-path-provisioner)
