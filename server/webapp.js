// Copyright 2017 Jacob Hesch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const ON = '01';
const OFF = '00';

// Used to avoid sending messages during programmatic UI updates.
var updatingUI = false;

function enableWidgets(enabled) {
  var state = enabled ? 'enable' : 'disable';
  $('#mute').flipswitch(state);
  $('#volume').slider(state);
  $('#input').selectmenu(state)
}

function initializeState() {
  $.ajax({
    url: '/integra',
    dataType: 'json',

    success: function(result) {
      updatingUI = true;
      if ('PWR' in result) {
        $('#power').prop('checked', (result.PWR == ON)).flipswitch('refresh');
      }
      if ('AMT' in result) {
        $('#mute').prop('checked', (result.AMT == ON)).flipswitch('refresh');
      }
      if ('MVL' in result) {
        var volume = parseInt(result.MVL, 16);
        $('#volume').val(volume).slider('refresh');
      }
      if ('SLI' in result) {
        $('#input').val(result.SLI).selectmenu('refresh');
      }
      // Always enable power, but only enable other widgets if power is on.
      enableWidgets($('#power').prop('checked'));
      $('#power').flipswitch('enable');
      updatingUI = false;
    },

    error: function(xhr, textStatus) {
      alert('Checking Integra state failed: ' + textStatus);
    },
  });
}

function dec2hex(dec) {
  var result = Math.round(dec).toString(16).toUpperCase();
  if (dec >= 0 && dec < 16) {
    result = "0" + result;
  }
  return result;
}

$(document).on('pagecreate', '#main', function() {

  if (!window.WebSocket) {
    alert('Your browser does not support WebSockets');
    return;
  }

  initializeState();

  var conn = new WebSocket('ws://' + document.location.host + '/ws');

  conn.onerror = function(event) {
    alert('Websocket error');
  };

  conn.onmessage = function(event) {
    var message = JSON.parse(event.data);
    updatingUI = true;

    switch(message.Command) {
    case 'PWR':
      $('#power').prop('checked', (message.Parameter == ON)).flipswitch('refresh');
      enableWidgets(message.Parameter == ON);
      break;
    case 'MVL':
      var volume = parseInt(message.Parameter, 16);
      $('#volume').val(volume).slider('refresh');
      break;
    case 'AMT':
      $('#mute').prop('checked', (message.Parameter == ON)).flipswitch('refresh');
      break;
    case 'SLI':
      $('#input_' + message.Parameter).prop('selected', true);
      $('#input').selectmenu('refresh');
      break;
    }
    updatingUI = false;
  };

  function sendMessage(command, parameter) {
    conn.send(JSON.stringify({
      Command: command,
      Parameter: parameter,
    }));
  }

  $('#power').on('change', function(event) {
    if (updatingUI) return;
    sendMessage('PWR', this.checked ? ON : OFF);
  });

  $('#mute').on('change', function(event) {
    if (updatingUI) return;
    sendMessage('AMT', this.checked ? ON : OFF);
  });

  $('#volume').on('slidestop', function(event) {
    if (updatingUI) return;
    sendMessage('MVL', dec2hex(this.value));
  });

  $('#input').on('change', function(event) {
    if (updatingUI) return;
    sendMessage('SLI', this.value);
  });

});
