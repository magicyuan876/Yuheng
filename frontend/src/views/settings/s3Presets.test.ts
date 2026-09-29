import assert from "node:assert/strict";
import { test } from "vitest";
import { S3_PRESETS, applyS3Preset, findS3Preset } from "./s3Presets";

const preset = (id: string) => {
  const found = findS3Preset(id);
  assert.ok(found, `preset ${id} exists`);
  return found;
};

test("presets cover the documented vendors with unique ids", () => {
  assert.deepEqual(
    S3_PRESETS.map((p) => p.id),
    ["rustfs", "minio", "aws", "oss", "cos", "tos", "obs"],
  );
  assert.equal(new Set(S3_PRESETS.map((p) => p.id)).size, S3_PRESETS.length);
});

test("vendors that require virtual-hosted addressing say so", () => {
  for (const id of ["oss", "cos", "tos", "obs"]) assert.equal(preset(id).addressingStyle, "virtual", id);
  for (const id of ["rustfs", "minio"]) assert.equal(preset(id).addressingStyle, "path", id);
  assert.equal(preset("aws").addressingStyle, "auto");
});

test("region placeholder resolves to the preset default when the form has no region", () => {
  assert.deepEqual(applyS3Preset(preset("oss")), {
    endpoint: "https://oss-cn-hangzhou.aliyuncs.com",
    region: "cn-hangzhou",
    use_ssl: true,
    addressing_style: "virtual",
  });
});

test("a region already typed wins over the preset default", () => {
  const fields = applyS3Preset(preset("cos"), "  ap-shanghai ");
  assert.equal(fields.region, "ap-shanghai");
  assert.equal(fields.endpoint, "https://cos.ap-shanghai.myqcloud.com");
});

test("plain-http self-hosted presets turn TLS off, AWS leaves the endpoint empty", () => {
  assert.equal(applyS3Preset(preset("minio")).use_ssl, false);
  const aws = applyS3Preset(preset("aws"));
  assert.equal(aws.endpoint, "");
  assert.equal(aws.use_ssl, true);
});

test("every endpoint template is fully resolved after applying", () => {
  for (const p of S3_PRESETS) assert.ok(!applyS3Preset(p).endpoint.includes("<"), p.id);
});
