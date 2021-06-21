import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { Ref, Project, UpsertProjectRequest, Variable } from 'waypoint-pb';
import { inject as service } from '@ember/service';
import ApiService from 'waypoint/services/api';

interface ProjectSettingsArgs {
  project;
}

export default class ProjectInputVariablesListComponent extends Component<ProjectSettingsArgs> {
  @service api!: ApiService;
  @service flashMessages;
  @tracked args;
  @tracked project;
  @tracked variablesList: Array<Variable.AsObject>;
  @tracked isCreating: boolean;
  @tracked activeVariable;

  constructor(owner: any, args: any) {
    super(owner, args);
    let { project } = args;
    this.project = project;
    this.variablesList = this.project.variablesList;
    this.activeVariable = null;
    this.isCreating = false;
  }

  @action
  addVariable() {
    this.isCreating = true;
    let newVar = new Variable();
    // why is this empty?
    newVar.setServer();
    let newVarObj = newVar.toObject();
    this.variablesList = [newVarObj, ...this.variablesList];
    this.activeVariable = newVarObj;
  }

  @action
  async deleteVariable(variable) {
    let existingVarIndex = this.variablesList.findIndex((v) => v.name === variable.name);
    if (existingVarIndex !== -1) {
      this.variablesList.splice(existingVarIndex, 1);
      this.variablesList = [...this.variablesList];
    }
    await this.saveVariableSettings();
  }

  @action
  cancelCreate() {
    this.activeVariable = null;
    this.isCreating = false;
  }

  @action
  async saveVariableSettings(variable?: Variable.AsObject) {
    let project = this.project;
    let ref = new Project();
    ref.setName(project.name);
    if (variable) {
      let existingVar = this.variablesList.find((v) => v.name === variable.name);
      if (existingVar) {
        existingVar = variable;
      }
    }
    let varlist = this.variablesList.map((v: Variable.AsObject) => {
      let variable = new Variable();
      variable.setName(v.name);
      variable.setStr(v.str);
      if (v.hcl) {
        variable.setHcl(v.hcl);
      }
      return variable;
    });
    ref.setVariablesList(varlist);
    // Build and trigger request
    let req = new UpsertProjectRequest();
    req.setProject(ref);
    try {
      let resp = await this.api.client.upsertProject(req, this.api.WithMeta());
      this.project = resp.toObject().project;
      this.flashMessages.success('Settings saved');
    } catch (err) {
      this.flashMessages.error('Failed to save Settings', { content: err.message, sticky: true });
    }
  }
}
