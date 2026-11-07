---
name: file-upload-weaponize
description: "Test upload validation, naming, storage, serving, and execution boundaries with disposable files"
---

# File upload boundary assessment

Adapted from the KSIR-ARTEX upload playbook (`240ae7c`). The source covered extension and MIME mismatches, double extensions, path handling, file inclusion, and PHP execution chains. Use a disposable account and inert marker files in the declared environment. An upload accepted by the server is a result to investigate, not proof of code execution.

## Map the upload path

1. Capture the normal upload request and the advertised type, size, and account rules. Record the server-assigned object ID, filename, storage bucket or path, download route, and cleanup route.
2. Establish the baseline with a small harmless file of an allowed type. Fetch it through the normal app route and record Content-Type, Content-Disposition, cache headers, access control, and whether the server changed its contents.
3. Build a bounded matrix, changing one variable per file: declared MIME versus actual bytes, case of the extension, double extension, URL-encoded filename, multiple dots, and an unexpected but inert text type. Keep file contents harmless and unique so storage effects can be traced.
4. Check parser and image-processing behavior where applicable. Compare the returned file hash or normalized output against the submitted input. A conversion error may reveal an unsupported parser, not an exploit.
5. Check another designated test account's ability to read, replace, or delete the object. Verify the committed state and audit event, not just the HTTP response.

## Establish impact

A public retrieval route may expose data; a same-origin active-content response may create a browser trust issue; path normalization may expose a different object's contents. For a suspected server execution path, identify the exact interpreter, inclusion or serving configuration, and a separate isolated lab proof using a harmless marker. Do not infer RCE from `.php` being accepted, a URL containing a script extension, or a 200 response. Record how the test file was removed and verify cleanup from the storage state.

The source PHP shell and bulk-exploit files are preserved under `extras/source-assets/php2shell` for provenance. They are not loaded as runtime skills. The active procedure keeps the original validation and path hypotheses and requires separate evidence for each claimed effect.
