<template>
  <q-card>
    <q-card-section class="row items-center q-pb-none">
      <div class="text-h6">Delete Task Group "{{ props.taskGroup.name }}"</div>
      <q-space />
      <q-btn v-if="closable" icon="close" flat round dense v-close-popup />
    </q-card-section>

    <q-card-section>
      Are you sure you want to delete this Task Group? All tasks that it contain will be stopped and destroyed.
    </q-card-section>

    <q-card-actions vertical>
      <q-btn v-if="closable" @click="cancel" label="Cancel" />
      <q-btn
        color="red"
        @click="deleteTaskGroup"
        :loading="busy"
        :disable="busy"
        label="Confirm Delete Task Group"
      />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useTaskGroupStore, TaskGroup } from 'src/stores/task-group-store'
import { Notify } from 'quasar'
import notifyError from 'src/lib/notifyError'

export interface Props {
  taskGroup: TaskGroup
  closable?: boolean
}
const props = withDefaults(defineProps<Props>(), { closable: true })

const emit = defineEmits(['onDelete', 'onCancel'])

const taskGroupStore = useTaskGroupStore()
const busy = ref(false)

function cancel () {
  emit('onCancel')
}

async function deleteTaskGroup () {
  try {
    busy.value = true
    await taskGroupStore.deleteTaskGroup(props.taskGroup.id)
    emit('onDelete')
    Notify.create({
      type: 'positive',
      position: 'top',
      message: 'Task Group successfully deleted!'
    })
  } catch (e) {
    notifyError(e)
  } finally {
    busy.value = false
  }
}
</script>
