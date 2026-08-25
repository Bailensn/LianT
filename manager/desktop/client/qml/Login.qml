import QtQuick
import QtQuick.Controls

Rectangle {
    id: root
    color: "#F4F5F7"

    property bool loggingIn: false

    function performLogin() {
        var ok = bridge.login(userField.text, passField.text)
        if (!ok) {
            errorLabel.visible = true
        }
    }

    Column {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.8, 340)
        spacing: 12

        Text {
            text: "LianT"
            font.pixelSize: 28
            font.bold: true
            color: "#2B2F36"
            anchors.horizontalCenter: parent.horizontalCenter
        }

        TextField {
            id: userField
            width: parent.width
            placeholderText: "Username or email"
            height: 40
        }

        TextField {
            id: passField
            width: parent.width
            height: 40
            placeholderText: "Password"
            echoMode: TextInput.Password
            onAccepted: root.performLogin()
        }

        Button {
            id: loginBtn
            width: parent.width
            height: 42
            text: "Sign in"
            onClicked: root.performLogin()
        }

        Text {
            id: errorLabel
            visible: false
            text: "Login failed. Check your credentials."
            color: "#D33"
            font.pixelSize: 12
            anchors.horizontalCenter: parent.horizontalCenter
        }
    }
}