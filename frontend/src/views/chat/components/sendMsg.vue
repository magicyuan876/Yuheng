<template>
  <div>
    <t-textarea
      resize="none"
      :autosize="false"
      v-model="value"
      :placeholder="$t('chat.enterDescription')"
      name="description"
      @change="onChange"
    />
  </div>
</template>
<script setup>
import { ref } from "vue";

const value = ref("");
const onChange = (value, e) => {
  console.log(value);
};
</script>
<style lang="less">
.chat {
  width: 800px;
  font-size: 20px;
  margin: 0px auto;
}
</style>
