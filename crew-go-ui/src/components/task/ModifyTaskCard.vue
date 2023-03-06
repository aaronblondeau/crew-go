<template>
  <q-card>
    <q-card-section class="row items-center q-pb-none">
      <div class="text-h6">
        {{ props.task ? "Edit" : "Create" }}
        <span v-if="!props.task && props.parentId">Child</span>
        Task
      </div>
      <q-space />
      <q-btn v-if="closable" icon="close" flat round dense v-close-popup />
    </q-card-section>

    <q-card-section>
      <q-form ref="modelForm">

        <q-input
          filled
          v-model="id"
          type="text"
          label="Id"
          :hint="props.task ? 'Id cannot be changed' : 'If left blank a uniqid will be used.'"
          class="q-mt-md"
          :readonly="!!props.task"
        />

        <q-input
          filled
          v-model="name"
          type="text"
          label="Name"
          :rules="[
            (val) => (val && val.length > 0) || 'Name must be filled in.',
          ]"
          class="q-mt-md"
          autofocus
        />

        <q-input
          filled
          v-model="worker"
          type="text"
          label="Worker"
          :rules="[
            (val) => (val && val.length > 0) || 'Worker must be filled in.',
          ]"
          class="q-mt-md"
        />

        <q-input
          filled
          v-model="workgroup"
          type="text"
          label="Workgroup"
          class="q-mt-md"
          hint="Workgroups are used to pause similar tasks. For example, when a rate limit occurs, delay all tasks using the same access token."
        />

        <q-input
          filled
          v-model="key"
          type="text"
          label="Key"
          class="q-mt-md"
          hint="Key is used to prevent duplicate tasks. When only one instance of a task should be executed use key to prevent duplicates from running."
        />

        <q-input
          filled
          v-model.number="remainingAttempts"
          type="number"
          label="Remaining Attempts"
          class="q-mt-md"
          hint="How many attempts should the task be tried."
        />

        <div class="q-mt-md">
          <q-checkbox v-model="isPaused" label="Paused" />
        </div>

        <div class="q-mt-md">
          <q-checkbox v-model="isComplete" label="Complete" />
        </div>

        TODO - if creating and props.parentId - autofill parentids (readonly) with {{ props.parentId }}

      </q-form>
    </q-card-section>

    <q-card-actions>
      <q-btn
        @click="create"
        color="primary"
        class="full-width q-mt-md"
        :loading="busy"
        :disable="busy"
        :label="(props.task ? 'Save' : 'Create') + ' Task'"
        />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
import { useTaskStore, Task, ModifyTask } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'
import { onMounted, ref, watch } from 'vue'
import notifyError from 'src/lib/notifyError'
import { Notify, QForm } from 'quasar'

export interface Props {
  task?: Task | null
  taskGroup: TaskGroup
  closable?: boolean
  parentId?: string
}
const props = withDefaults(defineProps<Props>(), { task: null, closable: true, parentId: '' })

const emit = defineEmits(['onCreate', 'onUpdate'])
const busy = ref(false)
const taskStore = useTaskStore()
const modelForm = ref<QForm | null>(null)

const name = ref('')
const worker = ref('')
const id = ref('')
const workgroup = ref('')
const key = ref('')
const remainingAttempts = ref(5)
const isPaused = ref(false)
const isComplete = ref(false)

function reset () {
  name.value = ''
  worker.value = ''
  workgroup.value = ''
  key.value = ''
  remainingAttempts.value = 5
  isPaused.value = false
  isComplete.value = false
}

async function create () {
  if (!modelForm.value) {
    return
  }
  const valid = await modelForm.value.validate()
  if (valid) {
    busy.value = true
    try {
      const payload: ModifyTask = {
        name: name.value,
        worker: worker.value,
        workgroup: workgroup.value,
        key: key.value,
        remainingAttempts: remainingAttempts.value,
        isPaused: isPaused.value,
        isComplete: isComplete.value
      }
      if (props.task) {
        // Updating existing task
        const updatedTask = await taskStore.updateTask(props.task.taskGroupId, props.task.id, payload)
        emit('onUpdate', updatedTask)
        Notify.create({
          type: 'positive',
          position: 'top',
          message: 'Task successfully saved!'
        })
      } else {
        // Creating new Task
        payload.id = id.value
        const newTask = await taskStore.createTask(props.taskGroup.id, payload)
        reset()
        emit('onCreate', newTask)
        Notify.create({
          type: 'positive',
          position: 'top',
          message: 'Task successfully created!'
        })
      }
    } catch (e) {
      notifyError(e)
    } finally {
      busy.value = false
    }
  }
}

function initFields () {
  if (props.task) {
    id.value = props.task.id
    name.value = props.task.name
    worker.value = props.task.worker
    workgroup.value = props.task.workgroup
    key.value = props.task.key
    remainingAttempts.value = props.task.remainingAttempts
    isPaused.value = props.task.isPaused
    isComplete.value = props.task.isComplete
  }
}

onMounted(async () => {
  initFields()
})

watch(
  () => props.task,
  () => {
    initFields()
  }
)
</script>
