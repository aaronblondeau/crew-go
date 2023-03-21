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

        <div class="q-mt-md">
          <div>
            <q-checkbox v-model="isSeed" label="Seed" />
          </div>
          <div class="text-caption">
            When a task group contains seed tasks all non-seed tasks are deleted when the task group is reset.
            Ths Seed option should usually only be checked for tasks that create child tasks.
          </div>
        </div>

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

        <q-input
          filled
          v-model.number="errorDelayInSeconds"
          type="number"
          label="Error Delay (Seconds)"
          class="q-mt-md"
          hint="How long to wait before retrying a task."
        />

        <div class="q-mt-md">
          <q-checkbox v-model="isPaused" label="Paused" />
        </div>

        <div class="q-mt-md">
          <q-checkbox v-model="isComplete" label="Complete" />
        </div>

        <q-input filled v-model="runAfter" label="Run After" class="q-mt-md" :readonly="true" @click="showRunAfterPopup">
          <template v-slot:append>
            <q-btn icon="close" flat @click="runAfter = ''" />
            <q-icon name="event" class="cursor-pointer">
              <q-popup-proxy ref="runAfterProxy" cover transition-show="scale" transition-hide="scale">
                <div class="row items-center justify-end">
                  <q-btn v-close-popup label="Close" icon="close" color="primary" flat />
                </div>
                <div class="row q-gutter-md items-start">
                  <q-date v-model="runAfter" :mask="dateFormat" />
                  <q-time v-model="runAfter" :mask="dateFormat" />
                </div>
              </q-popup-proxy>
            </q-icon>
          </template>
        </q-input>

        <div class="q-mt-md">
          <div class="text-h6">Input</div>
          <JsonEditorVue mode="text" v-model="input" ref="jsonEditor" />
        </div>

        <div class="q-mt-md">
          <div class="text-h6">Parent Ids</div>
          <q-list bordered separator v-if="parentIds.length > 0">
            <q-item clickable v-ripple v-for="parentId in parentIds" :key="parentId">
              <q-item-section>
                <span v-if="parentId">{{ parentId }}</span>
              </q-item-section>
              <q-item-section side v-if="parentIds.length > 1">
                <q-btn flat icon="delete" @click="removeParentId(parentId)"></q-btn>
              </q-item-section>
            </q-item>
          </q-list>
          <q-btn flat label="Add Parent" icon="add" @click="showAddAddParentIdDialog = true" />
          <q-dialog v-model="showAddAddParentIdDialog">
            <q-card>
              <q-card-section class="row items-center q-pb-none">
                <div class="text-h6">Add Parent</div>
                <q-space />
                <q-btn icon="close" flat round dense v-close-popup />
              </q-card-section>

              <q-card-section>
                <q-input
                  filled
                  v-model="newParentId"
                  label="Parent Task Id"
                  :rules="[
                    (val) => (val && val.length > 0) || 'Parent id must be filled in.',
                  ]"
                />
              </q-card-section>
              <q-card-actions align="right">
                <q-btn flat label="Cancel" color="primary" v-close-popup />
                <q-btn flat label="Add" color="primary" @click="addParentId" />
              </q-card-actions>
            </q-card>
          </q-dialog>
        </div>

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
import { Notify, QForm, QPopupProxy } from 'quasar'
import JsonEditorVue from 'json-editor-vue'
import _ from 'lodash'

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
const runAfter = ref('')
const runAfterProxy = ref<QPopupProxy | null>(null)
const dateFormat = 'YYYY-MM-DD[T]HH:mm:ss.SSSZ'
const isSeed = ref(false)
const errorDelayInSeconds = ref(30)
const input = ref<any>({})
const parentIds = ref<Array<string>>([])
const newParentId = ref('')
const showAddAddParentIdDialog = ref(false)
const jsonEditor = ref<typeof JsonEditorVue | null>(null)

function reset () {
  name.value = ''
  worker.value = ''
  workgroup.value = ''
  key.value = ''
  remainingAttempts.value = 5
  isPaused.value = false
  isComplete.value = false
  runAfter.value = ''
  isSeed.value = false
  errorDelayInSeconds.value = 30
  input.value = {}
  parentIds.value = []
}

async function create () {
  if (!modelForm.value) {
    return
  }
  if (jsonEditor.value) {
    const inputValid = jsonEditor.value.jsonEditor.validate()
    if (inputValid) {
      Notify.create({
        type: 'negative',
        message: 'Check task input field!'
      })
      return
    }
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
        isComplete: isComplete.value,
        runAfter: runAfter.value,
        isSeed: isSeed.value,
        errorDelayInSeconds: errorDelayInSeconds.value,
        input: _.isString(input.value) ? JSON.parse(input.value) : input.value,
        parentIds: parentIds.value
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
        if (!payload.runAfter) {
          delete payload.runAfter
        }

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
  } else {
    Notify.create({
      type: 'negative',
      message: 'Check form for errors!'
    })
  }
}

function initFields () {
  console.log(props.task)
  if (props.task) {
    id.value = props.task.id
    name.value = props.task.name
    worker.value = props.task.worker
    workgroup.value = props.task.workgroup
    key.value = props.task.key
    remainingAttempts.value = props.task.remainingAttempts
    isPaused.value = props.task.isPaused
    isComplete.value = props.task.isComplete
    runAfter.value = props.task.runAfter.startsWith('000') ? '' : props.task.runAfter
    isSeed.value = props.task.isSeed
    errorDelayInSeconds.value = props.task.errorDelayInSeconds
    input.value = props.task.input
    parentIds.value = props.task.parentIds || []
  }
  // if creating and props.parentId - autofill
  if (!props.task && props.parentId) {
    parentIds.value = [props.parentId]
  }
}

function showRunAfterPopup () {
  if (runAfterProxy.value) {
    runAfterProxy.value.show()
  }
}

function removeParentId (parentId: string) {
  parentIds.value = _.without(parentIds.value, parentId)
}

function addParentId () {
  parentIds.value.push(newParentId.value)
  newParentId.value = ''
  showAddAddParentIdDialog.value = false
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
