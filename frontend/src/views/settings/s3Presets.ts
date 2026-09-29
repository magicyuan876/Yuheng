import type { S3AddressingStyle } from "@/api/system";

/**
 * Quick-fill presets for the S3 form. The backend knows exactly one S3 provider and
 * has no concept of "MinIO" or "Aliyun OSS"; these entries only save the operator from
 * looking up an endpoint template and remembering which vendors insist on
 * virtual-hosted addressing. Nothing here is sent to the server except the values the
 * form ends up holding.
 */
export interface S3Preset {
  id: string;
  /** Vendor names are proper nouns and stay untranslated. */
  label: string;
  /** May contain a `<region>` placeholder, resolved by {@link applyS3Preset}. Empty for AWS S3. */
  endpoint: string;
  /** Sensible region to start from when the form has none. */
  region: string;
  addressingStyle: S3AddressingStyle;
}

export const S3_PRESETS: readonly S3Preset[] = [
  { id: "rustfs", label: "RustFS", endpoint: "http://localhost:9000", region: "us-east-1", addressingStyle: "path" },
  { id: "minio", label: "MinIO", endpoint: "http://localhost:9000", region: "us-east-1", addressingStyle: "path" },
  { id: "aws", label: "AWS S3", endpoint: "", region: "us-east-1", addressingStyle: "auto" },
  {
    id: "oss",
    label: "Aliyun OSS",
    endpoint: "https://oss-<region>.aliyuncs.com",
    region: "cn-hangzhou",
    addressingStyle: "virtual",
  },
  {
    id: "cos",
    label: "Tencent COS",
    endpoint: "https://cos.<region>.myqcloud.com",
    region: "ap-guangzhou",
    addressingStyle: "virtual",
  },
  {
    id: "tos",
    label: "Volcengine TOS",
    endpoint: "https://tos-s3-<region>.volces.com",
    region: "cn-beijing",
    addressingStyle: "virtual",
  },
  {
    id: "obs",
    label: "Huawei OBS",
    endpoint: "https://obs.<region>.myhuaweicloud.com",
    region: "cn-north-4",
    addressingStyle: "virtual",
  },
];

export interface S3PresetFields {
  endpoint: string;
  region: string;
  use_ssl: boolean;
  addressing_style: S3AddressingStyle;
}

/**
 * Computes the form fields a preset prefills. A region the operator already typed wins
 * over the preset's default, so picking a vendor after entering "cn-shanghai" yields
 * that region's endpoint instead of overwriting the choice. Credentials, bucket and
 * path prefix are never touched: the caller merges only these four fields.
 */
export function applyS3Preset(preset: S3Preset, currentRegion?: string): S3PresetFields {
  const region = currentRegion?.trim() || preset.region;
  const endpoint = preset.endpoint.replaceAll("<region>", region);
  return {
    endpoint,
    region,
    // The endpoint carries its own scheme; keep the flag consistent with it. With no
    // endpoint (AWS) the SDK default is HTTPS.
    use_ssl: !endpoint.startsWith("http://"),
    addressing_style: preset.addressingStyle,
  };
}

export function findS3Preset(id: string): S3Preset | undefined {
  return S3_PRESETS.find((preset) => preset.id === id);
}
