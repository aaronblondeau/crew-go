<template>
  <q-card>
    <q-card-section class="row items-center q-pb-none">
      <div class="text-h6">
        {{ props.taskGroup ? "Edit" : "Create" }} Task Group
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
          :hint="props.taskGroup ? 'Id cannot be changed' : 'If left blank a uniqid will be used.'"
          class="q-mt-sm"
          autofocus
          :readonly="!!props.taskGroup"
        />

        <q-input
          filled
          v-model="name"
          type="text"
          label="Name"
          :rules="[
            (val) => (val && val.length > 0) || 'Name must be filled in.',
          ]"
          class="q-mt-sm"
          autofocus
        />

      </q-form>
    </q-card-section>

    <q-card-actions>
      <q-btn
        @click="create"
        color="primary"
        class="full-width q-mt-sm"
        :loading="busy"
        :disable="busy"
        :label="(props.taskGroup ? 'Save' : 'Create') + ' Task Group'"
        />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
import { useTaskGroupStore, TaskGroup } from 'src/stores/task-group-store'
import { onMounted, ref, watch } from 'vue'
import notifyError from 'src/lib/notifyError'
import { Notify, QForm } from 'quasar'

export interface Props {
  taskGroup?: TaskGroup | null
  closable?: boolean
}
const props = withDefaults(defineProps<Props>(), { taskGroup: null, closable: true })

const emit = defineEmits(['onCreate', 'onSave'])
const busy = ref(false)
const taskGroupStore = useTaskGroupStore()
const modelForm = ref<QForm | null>(null)

const name = ref('')
const id = ref('')

function reset () {
  name.value = ''
}

async function create () {
  if (!modelForm.value) {
    return
  }
  const valid = await modelForm.value.validate()
  if (valid) {
    busy.value = true
    try {
      const payload = {
        name: name.value
      }
      if (props.taskGroup) {
        // Updating existing taskGroup
        const updatedTaskGroup = await taskGroupStore.updateTaskGroup(props.taskGroup.id, payload)
        emit('onSave', updatedTaskGroup)
        Notify.create({
          type: 'positive',
          position: 'top',
          message: 'Task Group successfully saved!'
        })
      } else {
        // Creating new Task Group
        const newTaskGroup = await taskGroupStore.createTaskGroup(id.value, name.value)
        reset()
        emit('onCreate', newTaskGroup)
        Notify.create({
          type: 'positive',
          position: 'top',
          message: 'Task Group successfully created!'
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
  if (props.taskGroup) {
    name.value = props.taskGroup.name
    id.value = props.taskGroup.id
  }
}

onMounted(async () => {
  initFields()
})

watch(
  () => props.taskGroup,
  () => {
    initFields()
  }
)
</script>
