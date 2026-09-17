App({
  onLaunch() {
    const settings = wx.getStorageSync("yuheng_settings");
    if (!settings) {
      wx.setStorageSync("yuheng_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: ""
      });
    }
  }
});
