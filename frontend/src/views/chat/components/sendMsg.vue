<template>
  <!-- The old global .chat rule matched no element (this root never carried the class), so the
       root stays unstyled. The textarea keeps the old fixed size: no resize handle, no autosize. -->
  <div>
    <Textarea
      v-model="value"
      class="field-sizing-fixed resize-none"
      :placeholder="$t('chat.enterDescription')"
      name="description"
      @change="onChange"
    />
  </div>
</template>
<script setup>
import { ref } from "vue";
import { Textarea } from "@/components/ui/textarea";

const value = ref("");
const onChange = (value, e) => {
  console.log(value);
};
</script>
