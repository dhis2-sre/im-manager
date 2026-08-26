# Postmortem: dhis2-core destroy deleted live MinIO volumes

**Date written:** 2026-08-26
**Status:** cause found, fix open in #1732, 30 affected volumes defused, re-adoption still to do
**Severity:** no data lost yet, but 30 production file stores were one pod reschedule away from being destroyed

## What happened

Destroying or resetting a `dhis2-core` instance deleted the persistent volume claim belonging to its sibling `minio` instance. Because the MinIO pod kept the claim mounted, Kubernetes could not finish the deletion, so the claim sat in `Terminating` with the `kubernetes.io/pvc-protection` finalizer holding it open. Thirty claims in `prod` had been in that state for up to four months, the oldest since 2026-04-16.

Nothing was visibly broken. The MinIO pods kept running and serving the file store from a volume already marked for destruction. The deletion would have completed the moment the pod moved for any reason: a node drain, an image change, a redeploy. At that point the volume, its reclaim policy set to `Delete`, would have been destroyed in Hetzner, and MinIO would have come back to a claim that no longer existed.

We watched exactly that happen to a different instance while investigating. `google-8-minio` was created at 10:59:49, marked for deletion at 11:09:07, and recreated at 11:10:50 on a new empty volume; the original was gone from Kubernetes and from Hetzner. That instance was minutes old so nothing of value was in it, but the sequence is the one the other thirty were queued up for.

## Why

`deletePersistentVolumeClaim` maps a stack name to the label selectors of the claims it should delete. The `dhis2-core` entry listed two:

```go
"dhis2-core": {"app.kubernetes.io/instance=%s", "app.kubernetes.io/instance=%s-minio"},
"minio":      {"app.kubernetes.io/instance=%s-minio"},
```

The second `dhis2-core` selector matches the MinIO claim, which belongs to the `minio` stack's own Helm release. The `minio` stack already lists it. Two different stacks claimed the same volume, and the `dhis2-core` stack does not deploy MinIO at all: its helmfile installs only `dhis2/core`.

Both entries arrived together in `849a43fa` ("MinIO and S3 support", #956, 2024-12-04), so the overlap has existed for as long as the MinIO stack has.

Destroying a whole deployment hid it. Every instance is destroyed, MinIO included, so whichever selector deleted the claim first, the result looked right. The overlap only bites when `dhis2-core` is destroyed on its own, which is what `Reset` and `DeleteInstance` do. `Reset` destroys one instance and immediately redeploys it, so `dhis2-core` comes back and MinIO is never touched, leaving its claim condemned and its pod running.

The affected instances are the ones that get reset regularly. `analytics-41-18` shows the shape: its `dhis2-core` release is revision 1 from 2026-08-26T04:05:47, a fresh install rather than an upgrade, while its MinIO release is untouched from 2026-08-13 and its claim was marked for deletion at 2026-08-14T04:07:35, two minutes into the same nightly window.

## Why nobody noticed

Three separate blind spots lined up.

The failure is invisible while it lasts. A `Terminating` claim with a running pod serves traffic normally. Nothing alerts on claims that have been terminating for months, and the instance looks healthy in IM because its pods are up.

The orphaned volume check does not report these, by design. It looks for claims nothing owns, and these are mounted by a running pod, so they are correctly not orphans. They are the opposite failure: a claim deleted too eagerly rather than not at all. That check was also dead for the 28 days before this investigation because the kubeconfig it uses had expired, and it said nothing about being dead.

The test suite asserted the bug. `TestDeletePersistentVolumeClaim` expected destroying `dhis2-core` to delete two claims, its own and MinIO's, so the behaviour was pinned in place as intended.

## Fix

`dhis2-core` no longer lists the MinIO claim (#1732). The `minio` stack's own entry already covers it, so full-deployment destroys still clean up, and resetting a core instance now leaves its sibling alone. The test that asserted the old behaviour is corrected, and a new one checks that destroying any stack leaves every other stack's claims intact, which is the general form of the defect rather than this one instance of it.

## Mitigation applied

All 30 volumes behind the wedged claims were patched from `persistentVolumeReclaimPolicy: Delete` to `Retain`. This changes nothing while the pods run and does not disturb them, but it means that when a claim does finalize, the volume survives as `Released` instead of being destroyed. The data-loss risk is gone; the claims are still wedged.

## Still to do

The 30 claims remain marked for deletion and will vanish when their pods move, at which point MinIO will have no claim to mount and the instance will need attention. Because the volumes are now retained, the data can be re-adopted rather than lost, but the sequence is manual: let the claim finalize, clear the volume's `claimRef`, create a fresh claim of the same name bound to that volume, and restart the MinIO deployment. It is worth doing deliberately, instance by instance, rather than waiting for a node drain to force it at an inconvenient moment.

## What this says about destroy

This is the third distinct way instance volume cleanup has failed, and all three are the same root problem: the claims are found by reconstructing a label selector from an instance name, and nothing checks the result against reality.

- A selector that matched nothing leaked volumes silently for a month after #1410, leaving 311 orphans and filling the volume quota. Now reported rather than silent (#1727) and detected daily (#1726).
- A selector belonging to the wrong stack deleted live volumes. This postmortem (#1732).
- The mechanism itself is unnecessary for the common case: Kubernetes will delete a StatefulSet's claims on its own if the retention policy says so (#1729), which removes the need to find them afterwards at all.

The durable answer is the one the roadmap already names under step 7: destroy should assert what it removed instead of assuming, and claims should be addressed by an identity they actually carry rather than by a name pattern rebuilt from somewhere else. Worth noting that the leaking claims carry no `im-*` labels at all, because `commonLabels` never reaches a `volumeClaimTemplate`, so today there is no identity to address them by.
