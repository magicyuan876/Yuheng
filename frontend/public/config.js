// 运行时配置（本地开发默认值，Docker 环境会被 entrypoint 脚本覆盖）
//
// Enterprise-edition promotion (both are set through container environment variables):
//   ENTERPRISE_INFO_URL        Page describing the Enterprise edition. Empty (default) shows no
//                              "learn more" link anywhere. Only absolute http(s) URLs are honoured.
//   HIDE_ENTERPRISE_PROMOTION  true removes the "Enterprise" settings section and every teaser for
//                              features that are not enabled. Default false. Enabled features are
//                              never hidden by it.
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: 50,
  // Optional: serve embed on a dedicated origin, e.g. 'https://embed.example.com'
  EMBED_BASE_URL: '',
  // Optional: default UI locale for first-time visitors (zh-CN | en-US | ru-RU | ko-KR)
  DEFAULT_LOCALE: '',
  ENTERPRISE_INFO_URL: '',
  HIDE_ENTERPRISE_PROMOTION: false,
};
