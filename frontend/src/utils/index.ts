import { MessagePlugin } from "tdesign-vue-next";
import i18n from "@/i18n";
import { shouldRejectKnowledgeFileType } from "./fileTypeVerification";

// 声明全局运行时配置类型
declare global {
  interface Window {
    __RUNTIME_CONFIG__?: {
      MAX_FILE_SIZE_MB?: number;
      MAX_VIDEO_FILE_SIZE_MB?: number;
      DEFAULT_LOCALE?: string;
    };
  }
}

// 上传大小上限是可运行时调整的系统设置（file.max_size_mb /
// file.video_max_size_mb，系统管理员在 UI 修改后立即生效）。这里的静态快照
// （容器启动时的 config.js > 构建时环境变量 > 默认值）只作为初始值；应用启动后
// 通过 GET /api/v1/system/upload-limits 刷新为实时值（见 api/system 的
// refreshUploadLimits，路由守卫首次进入时调用）。
let currentMaxFileSizeMB =
  window.__RUNTIME_CONFIG__?.MAX_FILE_SIZE_MB || Number(import.meta.env.VITE_MAX_FILE_SIZE_MB) || 50;

// 视频上传独立上限（默认 2048MB）：视频经共享存储卷直读，不受普通文件的
// gRPC 传输上限约束，因此允许远大于文档上限的体积。
let currentMaxVideoFileSizeMB =
  window.__RUNTIME_CONFIG__?.MAX_VIDEO_FILE_SIZE_MB || Number(import.meta.env.VITE_MAX_VIDEO_FILE_SIZE_MB) || 2048;

/** 当前生效的文档上传上限（MB）。 */
export function getMaxFileSizeMB(): number {
  return currentMaxFileSizeMB;
}
/** 当前生效的视频上传上限（MB）。 */
export function getMaxVideoFileSizeMB(): number {
  return currentMaxVideoFileSizeMB;
}

/** 用后端返回的实时值覆盖静态快照；非正数视为无效并忽略。 */
export function setUploadLimits(fileMB?: number, videoMB?: number) {
  if (typeof fileMB === "number" && Number.isFinite(fileMB) && fileMB > 0) {
    currentMaxFileSizeMB = fileMB;
  }
  if (typeof videoMB === "number" && Number.isFinite(videoMB) && videoMB > 0) {
    currentMaxVideoFileSizeMB = videoMB;
  }
}

const VIDEO_UPLOAD_EXTENSIONS = new Set(["mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v"]);

function isVideoFileName(name: string): boolean {
  const dot = name.lastIndexOf(".");
  if (dot < 0) return false;
  return VIDEO_UPLOAD_EXTENSIONS.has(name.substring(dot + 1).toLowerCase());
}

export function generateRandomString(length: number) {
  let result = "";
  const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  const charactersLength = characters.length;
  for (let i = 0; i < length; i++) {
    result += characters.charAt(Math.floor(Math.random() * charactersLength));
  }
  return result;
}

export function formatStringDate(date: any) {
  const data = new Date(date);
  const year = data.getFullYear();
  const month = String(data.getMonth() + 1).padStart(2, "0");
  const day = String(data.getDate()).padStart(2, "0");
  const hour = String(data.getHours()).padStart(2, "0");
  const minute = String(data.getMinutes()).padStart(2, "0");
  const second = String(data.getSeconds()).padStart(2, "0");
  return year + "-" + month + "-" + day + " " + hour + ":" + minute + ":" + second;
}
/** Returns true when the file exceeds the current upload limit. */
export function fileSizeVerification(file: Pick<File, "size"> & { name?: string }, silent = false) {
  const isVideo = !!file.name && isVideoFileName(file.name);
  const limitMB = isVideo ? currentMaxVideoFileSizeMB : currentMaxFileSizeMB;
  const limitBytes = limitMB * 1024 * 1024;
  if (file.size <= limitBytes) return false;
  if (!silent) {
    MessagePlugin.error(i18n.global.t("error.fileSizeExceeded", { size: limitMB }));
  }
  return true;
}

/**
 * Returns true when the file should be **rejected**.
 * @param validTypes - override the default extension whitelist with a dynamic set (e.g. from engine registry).
 */
export function kbFileTypeVerification(file: any, silent = false, validTypes?: Set<string> | string[]) {
  if (shouldRejectKnowledgeFileType(file.name, validTypes)) {
    if (!silent) {
      MessagePlugin.error(i18n.global.t("error.unsupportedFileType"));
    }
    return true;
  }
  return fileSizeVerification(file, silent);
}
