import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import ApiService from 'waypoint/services/api';
import { inject as service } from '@ember/service';

import { GetJobStreamRequest, GetJobStreamResponse } from 'waypoint-pb';
import { ITerminalOptions, Terminal } from 'xterm';

const ANSI_UI_GRAY_400 = '\x1b[38;2;142;150;163m';
const ANSI_WHITE = '\x1b[0m';

interface LogTerminalArgs {
  jobId: string;
}

export default class LogTerminal extends Component<LogTerminalArgs> {
  @service api!: ApiService;
  terminal: any;

  constructor(owner: any, args: any) {
    super(owner, args);
    let terminalOptions: ITerminalOptions = {
      fontFamily: 'monospace',
      fontWeight: '400',
      logLevel: 'debug',
    };
    let terminal = new Terminal(terminalOptions);
    this.terminal = terminal;
    this.start();
  }

  didInsertNode(element) {
    this.terminal.open(element);
    this.terminal.write(ANSI_UI_GRAY_400);
    this.terminal.writeln('Welcome to Waypoint...');
  }

  willDestroyNode() {
    this.terminal.dispose();
  }

  writeTerminalOutput(response: GetJobStreamResponse) {
    let event = response.getEventCase();
    console.log(event);
    if (event == GetJobStreamResponse.EventCase.STATE) {
      debugger;
    }
    if (event == GetJobStreamResponse.EventCase.TERMINAL) {
      let terminal = response.getTerminal();
      if (!terminal) {
        this.terminal.writeln('status', { msg: 'Logs are no longer available for this operation' });
      } else {
        terminal.getEventsList().forEach((event) => {
          let line = event.getLine();
          let step = event.getStep();

          if (line && line.getMsg()) {
            console.log(line.getMsg());
            this.terminal.writeln(line.getMsg());
          }

          if (step && step.getOutput()) {
            console.log(step);
            let newStep = step.toObject();

            if (step.getOutput_asU8().length > 0) {
              newStep.output = new TextDecoder().decode(step.getOutput_asU8());
            }

            this.terminal.writeUtf8(step.getOutput_asU8());
          }
        });
      }
    }
  }

  onData = (response: GetJobStreamResponse) => {
    console.log(response.getTerminal());
    this.writeTerminalOutput(response);
  }

  onStatus = (status: any) => {
    console.log(status);
    if (status.details) {
      this.terminal.writeln(status);
    }
  }

  async start() {
    let req = new GetJobStreamRequest();
    req.setJobId(this.args.jobId);
    let stream = this.api.client.getJobStream(req, this.api.WithMeta());

    stream.on('data', this.onData);
    stream.on('status', this.onStatus);
  }
}
