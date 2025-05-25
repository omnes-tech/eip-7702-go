// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function mint(address to, uint256 amount) external;
    function balanceOf(address account) external view returns (uint256);
}

contract SimpleDelegateContract {
    event Executed(address indexed to, uint256 value, bytes data);
    event TokenOperation(string operation, address token, address to, uint256 amount, bool success);

    struct Call {
        bytes data;
        address to;
        uint256 value;
    }

    function execute(Call[] memory calls) external payable {
        for (uint256 i = 0; i < calls.length; i++) {
            Call memory call = calls[i];
            (bool success, bytes memory result) = call.to.call{value: call.value}(call.data);
            require(success, string(result));
            emit Executed(call.to, call.value, call.data);
        }
    }

    // Função para enviar ETH 
    function sendETH(address payable to, uint256 amount) external {
        require(address(this).balance >= amount, "Insufficient balance");
        (bool success,) = to.call{value: amount}("");
        require(success, "ETH transfer failed");
        emit Executed(to, amount, "");
    }

    // Função para mintar tokens - NO CONTEXTO DO AUTHORITY
    function mint(address token, address to, uint256 amount) external {
        IERC20(token).mint(to, amount);
        emit TokenOperation("mint", token, to, amount, true);
    }

    // Função para transferir tokens - 
    function transfer(address token, address to, uint256 amount) external {
        // No contexto EIP-7702, msg.sender É o authority original
        // Então podemos transferir diretamente do authority
        bool success = IERC20(token).transfer(to, amount);
        require(success, "Token transfer failed");
        emit TokenOperation("transfer", token, to, amount, success);
    }

    // Função para transferFrom (se precisar de approval)
    function transferFrom(address token, address from, address to, uint256 amount) external {
        bool success = IERC20(token).transferFrom(from, to, amount);
        require(success, "TransferFrom failed");
        emit TokenOperation("transferFrom", token, to, amount, success);
    }

    receive() external payable {}
}