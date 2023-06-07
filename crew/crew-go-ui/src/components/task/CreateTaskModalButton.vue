<template>
  <q-btn @click="showCreateDialog = true" :size="props.size" :flat="props.flat" color="primary" icon="add" :label="props.label">
    <slot />
  </q-btn>

  <q-dialog v-model="showCreateDialog">
    <ModifyTaskCard :task-group="props.taskGroup" :parent-id="props.parentId" style="width: 700px; max-width: 80vw;" @on-create="onCreate" />
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ModifyTaskCard from 'src/components/task/ModifyTaskCard.vue'
import { Task } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'

export interface Props {
  flat?: boolean
  label?: string
  size?: string
  taskGroup: TaskGroup
  parentId?: string
}
const props = withDefaults(defineProps<Props>(), { label: 'New Task', flat: false, size: 'md' })

const emit = defineEmits(['onCreate'])

function onCreate (task: Task) {
  showCreateDialog.value = false
  emit('onCreate', task)
}

const showCreateDialog = ref(false)
</script>
