<template>
  <q-card>
    <q-card-section class="row items-center q-pb-none">
      <div class="text-h6">Delete Task "{{ props.task.name }}"</div>
      <q-space />
      <q-btn v-if="closable" icon="close" flat round dense v-close-popup />
    </q-card-section>

    <q-card-section>
      Are you sure you want to delete this Task?
    </q-card-section>

    <q-card-actions vertical>
      <q-btn v-if="closable" @click="cancel" label="Cancel" />
      <q-btn
        color="red"
        @click="deleteTask"
        :loading="busy"
        :disable="busy"
        label="Confirm Delete Task"
      />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useTaskStore, Task } from 'src/stores/task-store'
import { Notify } from 'quasar'
import notifyError from 'src/lib/notifyError'

export interface Props {
  task: Task
  closable?: boolean
}
const props = withDefaults(defineProps<Props>(), { closable: true })

const emit = defineEmits(['onDelete', 'onCancel'])

const taskStore = useTaskStore()
const busy = ref(false)

function cancel () {
  emit('onCancel')
}

async function deleteTask () {
  try {
    busy.value = true
    await taskStore.deleteTask(props.task.taskGroupId, props.task.id)
    emit('onDelete')
    Notify.create({
      type: 'positive',
      position: 'top',
      message: 'Task successfully deleted!'
    })
  } catch (e) {
    notifyError(e)
  } finally {
    busy.value = false
  }
}
</script>
