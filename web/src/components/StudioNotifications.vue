<script setup lang="ts">
import {
  notifications,
  pendingActions,
  dismissToast,
} from "../composables/useNotifications";
import { LoaderCircle, X, CircleAlert, Check } from "lucide-vue-next";
</script>
<template>
  <div class="studio-feedback" aria-live="polite" aria-relevant="additions">
    <div v-if="pendingActions.size" class="studio-toast pending" role="status">
      <LoaderCircle class="spin" :size="18" /><span
        >{{
          [...pendingActions]
            .map(([scope, action]) => `${scope}: ${action}`)
            .join(" · ")
        }}…</span
      >
    </div>
    <div
      v-for="n in notifications"
      :key="n.id"
      class="studio-toast"
      :class="n.kind"
      :role="n.kind === 'error' ? 'alert' : 'status'"
    >
      <CircleAlert v-if="n.kind === 'error'" :size="18" /><Check
        v-else
        :size="18"
      /><span>{{ n.text }}</span
      ><button @click="dismissToast(n.id)" aria-label="Dismiss notification">
        <X :size="16" />
      </button>
    </div>
  </div>
</template>
<style scoped>
.studio-feedback {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 12000;
  display: grid;
  gap: 8px;
  width: min(440px, calc(100vw - 32px));
  pointer-events: none;
}
.studio-toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 16px;
  border-radius: 10px;
  background: #123d34;
  color: #fff;
  box-shadow: 0 6px 24px #16364a33;
  font-size: 13px;
  line-height: 1.5;
}
.studio-toast > svg {
  flex-shrink: 0;
}
.studio-toast span {
  flex: 1;
}
.studio-toast button {
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
}
.studio-toast.error {
  background: #8e3029;
}
.studio-toast.pending {
  background: #244b74;
}
.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .spin {
    animation: none;
  }
}
</style>
