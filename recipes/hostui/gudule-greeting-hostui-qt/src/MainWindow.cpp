#include "MainWindow.h"

#include <QVBoxLayout>
#include <QWidget>

MainWindow::MainWindow(QWidget *parent)
    : QMainWindow(parent), daemon_("gudule-daemon-greeting-goqt"),
      statusLabel_(new QLabel(this)),
      languageCombo_(new QComboBox(this)), nameEdit_(new QLineEdit(this)),
      helloButton_(new QPushButton(QStringLiteral("Greet"), this)),
      greetingLabel_(new QLabel(this)) {
  auto *root = new QWidget(this);
  auto *layout = new QVBoxLayout(root);
  auto *languageLabel = new QLabel(QStringLiteral("Language"), this);

  setWindowTitle(QStringLiteral("Gudule Greeting Goqt"));
  resize(640, 480);

  statusLabel_->setWordWrap(true);
  languageCombo_->setObjectName(QStringLiteral("language-picker"));
  nameEdit_->setPlaceholderText(QStringLiteral("Your name"));
  nameEdit_->setObjectName(QStringLiteral("name-input"));
  nameEdit_->setText(QStringLiteral("World"));
  helloButton_->setObjectName(QStringLiteral("greet-button"));
  greetingLabel_->setWordWrap(true);
  greetingLabel_->setObjectName(QStringLiteral("greeting-output"));
  greetingLabel_->setMinimumHeight(120);

  layout->addWidget(statusLabel_);
  layout->addWidget(languageLabel);
  layout->addWidget(languageCombo_);
  layout->addWidget(nameEdit_);
  layout->addWidget(helloButton_);
  layout->addWidget(greetingLabel_);
  setCentralWidget(root);

  connect(helloButton_, &QPushButton::clicked, this, &MainWindow::sayHello);

  initialize();
}

MainWindow::~MainWindow() { daemon_.stop(); }

void MainWindow::initialize() {
  if (!daemon_.start()) {
    statusLabel_->setText(QStringLiteral("Failed to start daemon: %1").arg(daemon_.lastError()));
    helloButton_->setEnabled(false);
    return;
  }

  client_.configure(daemon_.binaryPath(), daemon_.grpcTarget());
  statusLabel_->setText(
      QStringLiteral("Daemon connected via %1 (%2)")
          .arg(daemon_.target(), daemon_.grpcTarget()));
  refreshLanguages();
}

void MainWindow::refreshLanguages() {
  const auto languages = client_.listLanguages();
  if (languages.isEmpty()) {
    statusLabel_->setText(
        QStringLiteral("Daemon started, but language loading is unavailable: %1")
            .arg(client_.lastError()));
    return;
  }

  languageCombo_->clear();
  int englishIndex = -1;
  int index = 0;
  for (const auto &language : languages) {
    languageCombo_->addItem(
        QStringLiteral("%1 (%2)").arg(language.nativeName, language.name),
        language.code);
    if (language.code == QStringLiteral("en")) {
      englishIndex = index;
    }
    ++index;
  }

  if (languageCombo_->count() > 0) {
    languageCombo_->setCurrentIndex(englishIndex >= 0 ? englishIndex : 0);
  }
}

void MainWindow::sayHello() {
  if (languageCombo_->currentIndex() < 0) {
    greetingLabel_->setText(QStringLiteral("Select a language first."));
    return;
  }

  const QString greeting =
      client_.sayHello(nameEdit_->text().trimmed(), languageCombo_->currentData().toString());
  if (greeting.isEmpty()) {
    greetingLabel_->setText(client_.lastError());
    return;
  }

  greetingLabel_->setText(greeting);
}
